// mystack-controller api
// +build unit
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/mystack-controller/models"

	"database/sql"
	"github.com/jmoiron/sqlx"
	mTest "github.com/topfreegames/mystack-controller/testing"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/labels"
)

var _ = Describe("Cluster", func() {
	const (
		yaml1 = `
setup:
  image: setup-img
services:
  test0:
    image: svc1
    ports: 
      - "5000"
      - "5001:5002"
    readinessProbe:
      command:
        - echo
        - ready
apps:
  test1:
    image: app1
    ports: 
      - "5000"
      - "5001:5002"
  test2:
    image: app2
    ports: 
      - "5000"
      - "5001:5002"
  test3:
    image: app3
    ports: 
      - "5000"
      - "5001:5002"
    env:
      - name: VARIABLE_1
        value: 100
`
	)
	var (
		db          *sql.DB
		sqlxDB      *sqlx.DB
		mock        sqlmock.Sqlmock
		err         error
		clusterName = "MyCustomApps"
		clientset   *fake.Clientset
		username    = "user"
		namespace   = "mystack-user"
		ports       = []int{5000, 5002}
		portMaps    = []*PortMap{
			&PortMap{Port: 5000, TargetPort: 5000},
			&PortMap{Port: 5001, TargetPort: 5002},
		}
		labelMap    = labels.Set{"mystack/routable": "true"}
		listOptions = v1.ListOptions{
			LabelSelector: labelMap.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	mockCluster := func(username string) *Cluster {
		return &Cluster{
			Username:  username,
			Namespace: namespace,
			AppDeployments: []*Deployment{
				NewDeployment("test1", username, "app1", ports, nil, nil),
				NewDeployment("test2", username, "app2", ports, nil, nil),
				NewDeployment("test3", username, "app3", ports, []*EnvVar{
					&EnvVar{Name: "VARIABLE_1", Value: "100"},
				}, nil),
			},
			SvcDeployments: []*Deployment{
				NewDeployment("test0", username, "svc1", ports, nil, &Probe{Command: []string{"echo", "ready"}}),
			},
			AppServices: []*Service{
				NewService("test1", username, portMaps),
				NewService("test2", username, portMaps),
				NewService("test3", username, portMaps),
			},
			SvcServices: []*Service{
				NewService("test0", username, portMaps),
			},
			Setup: NewJob(username, "setup-img", []*EnvVar{
				&EnvVar{Name: "VARIABLE_1", Value: "100"},
			}),
			DeploymentReadiness: &mTest.MockReadiness{},
			JobReadiness:        &mTest.MockReadiness{},
		}
	}

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("NewCluster", func() {
		BeforeEach(func() {
			db, mock, err = sqlmock.New()
			Expect(err).NotTo(HaveOccurred())
			sqlxDB = sqlx.NewDb(db, "postgres")
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
			db.Close()
		})

		It("should return cluster from config on DB", func() {
			mockedCluster := mockCluster(username)

			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(err).NotTo(HaveOccurred())
			Expect(cluster.AppDeployments).To(ConsistOf(mockedCluster.AppDeployments))
			Expect(cluster.SvcDeployments).To(ConsistOf(mockedCluster.SvcDeployments))
			Expect(cluster.SvcServices).To(ConsistOf(mockedCluster.SvcServices))
			Expect(cluster.AppServices).To(ConsistOf(mockedCluster.AppServices))
			Expect(cluster.Setup).To(Equal(mockedCluster.Setup))
		})

		It("should return error if clusterName doesn't exists on DB", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(cluster).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("sql: no rows in result set"))
		})

		It("should return error if empty clusterName", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}))

			cluster, err := NewCluster(sqlxDB, username, clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{})
			Expect(cluster).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("sql: no rows in result set"))
		})
	})

	Describe("Create", func() {
		It("should create cluster", func() {
			cluster := mockCluster(username)
			err := cluster.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))

			k8sJob, err := clientset.BatchV1().Jobs(namespace).Get("setup")
			Expect(err).NotTo(HaveOccurred())
			Expect(k8sJob).NotTo(BeNil())
			Expect(k8sJob.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(k8sJob.ObjectMeta.Name).To(Equal("setup"))
			Expect(k8sJob.ObjectMeta.Labels["mystack/owner"]).To(Equal(username))
			Expect(k8sJob.ObjectMeta.Labels["app"]).To(Equal("setup"))
			Expect(k8sJob.ObjectMeta.Labels["heritage"]).To(Equal("mystack"))
			Expect(k8sJob.Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("VARIABLE_1"))
			Expect(k8sJob.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("100"))
			Expect(k8sJob.Spec.Template.Spec.Containers[0].Image).To(Equal("setup-img"))
		})

		It("should return error if creating same cluster twice", func() {
			cluster := mockCluster(username)
			err := cluster.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster.Create(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Namespace \"mystack-user\" already exists"))
		})

		It("should run without setup image", func() {
			cluster := mockCluster(username)
			cluster.Setup = nil
			err := cluster.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))

			jobs, err := clientset.BatchV1().Jobs(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(jobs.Items).To(BeEmpty())
		})
	})

	Describe("Delete", func() {
		It("should delete cluster", func() {
			cluster := mockCluster(username)
			err := cluster.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(NamespaceExists(clientset, namespace)).To(BeFalse())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(BeEmpty())

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(BeEmpty())
		})

		It("should delete only specified cluster", func() {
			cluster1 := mockCluster("user1")
			err := cluster1.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			cluster2 := mockCluster("user2")
			err = cluster2.Create(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = cluster1.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			Expect(NamespaceExists(clientset, "mystack-user1")).To(BeFalse())
			Expect(NamespaceExists(clientset, "mystack-user2")).To(BeTrue())

			deploys, err := clientset.ExtensionsV1beta1().Deployments("mystack-user1").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(BeEmpty())

			services, err := clientset.CoreV1().Services("mystack-user1").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(BeEmpty())

			deploys, err = clientset.ExtensionsV1beta1().Deployments("mystack-user2").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(4))

			services, err = clientset.CoreV1().Services("mystack-user2").List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(4))
		})

		It("should return error when deleting non existing cluster", func() {
			cluster := mockCluster(username)

			err = cluster.Delete(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Service \"test1\" not found"))
		})
	})
})
