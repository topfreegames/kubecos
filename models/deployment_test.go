// mystack-controller api
// +build unit
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package models_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/mystack-controller/models"

	mTest "github.com/topfreegames/mystack-controller/testing"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/labels"
)

var _ = Describe("Deployment", func() {
	var (
		clientset   *fake.Clientset
		name        = "test"
		namespace   = "mystack-user"
		username    = "user"
		image       = "hello-world"
		ports       = []int{5000, 5001, 5002}
		labelMap    = labels.Set{"mystack/routable": "true"}
		listOptions = v1.ListOptions{
			LabelSelector: labelMap.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("Deploy", func() {
		It("should return error since namespace was not created", func() {
			deployment := NewDeployment(name, username, image, ports, nil, nil)
			_, err := deployment.Deploy(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("namespace mystack-user not found"))
		})

		It("should create a deployment", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			deployment := NewDeployment(name, username, image, ports, nil, nil)
			deploy, err := deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).NotTo(BeNil())
			Expect(deploy.ObjectMeta.Namespace).To(Equal(namespace))
			Expect(deploy.ObjectMeta.Name).To(Equal(name))
			Expect(deploy.ObjectMeta.Labels["mystack/owner"]).To(Equal(username))
			Expect(deploy.ObjectMeta.Labels["app"]).To(Equal(name))
			Expect(deploy.ObjectMeta.Labels["heritage"]).To(Equal("mystack"))

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(1))
		})

		It("should create deployment with environment variables", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			environment := []*EnvVar{
				&EnvVar{
					Name:  "DATABASE_URL",
					Value: "postgres://derp:1234@example.com",
				},
			}

			deployment := NewDeployment(name, username, image, ports, environment, nil)
			deploy, err := deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).NotTo(BeNil())
			Expect(deploy.Spec.Template.Spec.Containers[0].Env[0].Name).To(Equal("DATABASE_URL"))
			Expect(deploy.Spec.Template.Spec.Containers[0].Env[0].Value).To(Equal("postgres://derp:1234@example.com"))
		})

		It("should create deployment with readiness probe", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			probe := &Probe{
				Command: []string{"echo", "ready"},
			}

			deployment := NewDeployment(name, username, image, ports, nil, probe)
			deploy, err := deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).NotTo(BeNil())

			dr := &mTest.MockReadiness{}
			err = dr.WaitForCompletion(clientset, []*Deployment{deployment})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create deployment with readiness probe", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			probe := &Probe{
				Command: []string{"echo", "ready"},
			}

			deployment := NewDeployment(name, username, image, ports, nil, probe)
			deploy, err := deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploy).NotTo(BeNil())

			dr := &mTest.MockReadiness{}
			err = dr.WaitForCompletion(clientset, []*Deployment{deployment})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error if duplicate deployment", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			deployment := NewDeployment(name, username, image, ports, nil, nil)
			_, err = deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = deployment.Deploy(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Deployment.extensions \"test\" already exists"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.KubernetesError"))
		})

		It("should not return error if create second deployment on same namespace", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			deployment := NewDeployment(name, username, image, ports, nil, nil)
			_, err = deployment.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			deployment2 := NewDeployment("test2", username, "new-image", []int{5000}, nil, nil)
			_, err = deployment2.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(2))
		})
	})

	Describe("Delete", func() {
		It("should return error if deployment wasn't deployed", func() {
			deploy := NewDeployment(name, username, image, ports, nil, nil)
			err := deploy.Delete(clientset)
			Expect(err).To(HaveOccurred())
		})

		It("should delete deployment after deploy", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			deploy := NewDeployment(name, username, image, ports, nil, nil)
			_, err = deploy.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = deploy.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(0))
		})

		It("should not delete all deployments", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			deploy := NewDeployment(name, username, image, ports, nil, nil)
			_, err = deploy.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploy2 := NewDeployment("test2", username, image, ports, nil, nil)
			_, err = deploy2.Deploy(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = deploy.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			deploys, err := clientset.ExtensionsV1beta1().Deployments(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(deploys.Items).To(HaveLen(1))
		})
	})
})
