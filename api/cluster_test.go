// mystack-controller api
// +build unit
// https://github.com/topfreegames/mystack-controller
//
// Licensed under the MIT license:
// http://www.opensource.org/licenses/mit-license
// Copyright © 2017 Top Free Games <backend@tfgco.com>

package api_test

import (
	"encoding/json"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/topfreegames/mystack-controller/api"
	"github.com/topfreegames/mystack-controller/models"

	mTest "github.com/topfreegames/mystack-controller/testing"
	"gopkg.in/DATA-DOG/go-sqlmock.v1"
	"k8s.io/client-go/pkg/api/resource"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/labels"
	"net/http"
	"net/http/httptest"
)

var _ = Describe("Cluster", func() {

	var (
		recorder       *httptest.ResponseRecorder
		clusterName    = "myCustomApps"
		clusterHandler *ClusterHandler
		yaml1          = `
setup:
  image: setup-img
services:
  test0:
    image: svc1
    port: 5000
apps:
  test1:
    image: app1
    port: 5000
`
		yamlWithoutSetup = `
services:
  test0:
    image: svc1
    port: 5000
apps:
  test1:
    image: app1
    port: 5000
`
		yamlWithVolume = `
volumes:
  - name: postgres-volume
    storage: 1Gi
services:
  postgres:
    image: postgres:1.0
    ports:
      - 8585:5432
    env:
      - name: PGDATA
        value: /var/lib/postgresql/data/pgdata
    volumeMount:
      name: postgres-volume
      mountPath: /var/lib/postgresql/data
apps:
  app1:
    image: app1
    ports:
      - 5000:5001
    env:
      - name: DATABASE_URL
        value: postgresql://derp:1234@example.com
      - name: USERNAME
        value: derp
`
		yamlWithLimitsAndResources = `
apps:
  app1:
    image: app1
    ports:
      - 5000:5001
    resources:
      limits:
        cpu: "20m"
        memory: "600Mi"
      requests:
        cpu: "10m"
        memory: "200Mi"
`
		yamlWithLimits = `
apps:
  app1:
    image: app1
    ports:
      - 5000:5001
    resources:
      limits:
        cpu: "20m"
        memory: "600Mi"
`
	)

	BeforeEach(func() {
		recorder = httptest.NewRecorder()
		clusterHandler = &ClusterHandler{App: app}
	})

	Describe("PUT /clusters/{name}/create", func() {

		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/create", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "create"
			request, err = http.NewRequest("PUT", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create existing clusterName", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			bodyJSON := make(map[string]map[string][]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["domains"]["test1"]).To(Equal([]string{"test1.mystack-user.mystack.com"}))
		})

		It("should create existing clusterName without setup", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithoutSetup))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			bodyJSON := make(map[string]map[string][]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["domains"]["test1"]).To(Equal([]string{"test1.mystack-user.mystack.com"}))
		})

		It("should create existing clusterName with volume", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithVolume))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			bodyJSON := make(map[string]map[string][]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["domains"]["app1"]).To(Equal([]string{"app1.mystack-user.mystack.com"}))
		})

		It("should create cluster with requests and limits", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithLimitsAndResources))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Code).To(Equal(http.StatusOK))

			deploys, err := clientset.ExtensionsV1beta1().Deployments("mystack-user").List(v1.ListOptions{
				LabelSelector: labels.Set{"mystack/routable": "true"}.AsSelector().String(),
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())

			k8sDeploy := deploys.Items[0]
			limitCPU, _ := resource.ParseQuantity("20m")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"]).To(Equal(limitCPU))
			limitMemory, _ := resource.ParseQuantity("600Mi")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Limits["memory"]).To(Equal(limitMemory))
			requestCPU, _ := resource.ParseQuantity("10m")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"]).To(Equal(requestCPU))
			requestMemory, _ := resource.ParseQuantity("200Mi")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Requests["memory"]).To(Equal(requestMemory))
		})

		It("should create cluster with limits", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithLimits))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Code).To(Equal(http.StatusOK))

			deploys, err := clientset.ExtensionsV1beta1().Deployments("mystack-user").List(v1.ListOptions{
				LabelSelector: labels.Set{"mystack/routable": "true"}.AsSelector().String(),
				FieldSelector: fields.Everything().String(),
			})
			Expect(err).NotTo(HaveOccurred())

			k8sDeploy := deploys.Items[0]
			limitCPU, _ := resource.ParseQuantity("20m")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Limits["cpu"]).To(Equal(limitCPU))
			limitMemory, _ := resource.ParseQuantity("600Mi")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Limits["memory"]).To(Equal(limitMemory))
			requestCPU, _ := resource.ParseQuantity("5m")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Requests["cpu"]).To(Equal(requestCPU))
			requestMemory, _ := resource.ParseQuantity("100Mi")
			Expect(k8sDeploy.Spec.Template.Spec.Containers[0].Resources.Requests["memory"]).To(Equal(requestMemory))
		})

		It("should not create cluster twice", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			recorder = httptest.NewRecorder()
			request, _ = http.NewRequest("PUT", route, nil)
			ctx = NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["code"]).To(Equal("MST-004"))
			Expect(bodyJSON["description"]).To(Equal("namespace for user 'user' already exists"))
			Expect(bodyJSON["error"]).To(Equal("create cluster error"))
			Expect(recorder.Code).To(Equal(http.StatusConflict))
		})

		It("should return error 404 when create non existing clusterName", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnError(fmt.Errorf("sql: no rows in result set"))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["code"]).To(Equal("MST-003"))
			Expect(bodyJSON["description"]).To(Equal("sql: no rows in result set"))
			Expect(bodyJSON["error"]).To(Equal("database error"))
		})
	})

	Describe("PUT /clusters/{name}/delete", func() {

		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/delete", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "delete"
			request, err = http.NewRequest("PUT", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete existing clusterName", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			cluster, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())
			err = cluster.Create(app.Logger, app.Clientset)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Body.String()).To(Equal(`{"status": "ok"}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should delete existing clusterName with volumes", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithVolume))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yamlWithVolume))

			cluster, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())
			err = cluster.Create(app.Logger, app.Clientset)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Body.String()).To(Equal(`{"status": "ok"}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})

		It("should return error 404 when deleting non existing cluster", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnError(fmt.Errorf("sql: no rows in result set"))

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["description"]).To(Equal("namespace for user 'user' not found"))
			Expect(bodyJSON["error"]).To(Equal("delete cluster error"))
			Expect(bodyJSON["code"]).To(Equal("MST-004"))
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})

		It("should delete cluster even if cluster config doesn't exist anymore", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnError(fmt.Errorf("sql: no rows in result set"))

			cluster, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())
			err = cluster.Create(app.Logger, app.Clientset)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Body.String()).To(Equal(`{"status": "ok"}`))
			Expect(recorder.Code).To(Equal(http.StatusOK))
		})
	})

	Describe("GET /clusters/{name}/apps", func() {
		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/apps", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "apps"
			request, err = http.NewRequest("GET", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return correct apps", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			cluster, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())
			err = cluster.Create(app.Logger, app.Clientset)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			bodyJSON := make(map[string]map[string][]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["domains"]["test1"]).To(Equal([]string{"test1.mystack-user.mystack.com"}))
		})

		It("should return status 404 if namespace doesn't exist", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			_, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["description"]).To(Equal("namespace for user 'user' not found"))
			Expect(bodyJSON["error"]).To(Equal("get apps error"))
			Expect(bodyJSON["code"]).To(Equal("MST-004"))
		})
	})

	Describe("GET /clusters/{name}/services", func() {
		var (
			err     error
			request *http.Request
			route   = fmt.Sprintf("/clusters/%s/services", clusterName)
		)

		BeforeEach(func() {
			clusterHandler.Method = "services"
			request, err = http.NewRequest("GET", route, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			err = mock.ExpectationsWereMet()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return correct services", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			cluster, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())
			err = cluster.Create(app.Logger, app.Clientset)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			Expect(recorder.Code).To(Equal(http.StatusOK))
			bodyJSON := make(map[string][]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["services"]).To(Equal([]string{"test0"}))
		})

		It("should return status 404 if namespace doesn't exist", func() {
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))
			mock.
				ExpectQuery("^SELECT yaml FROM clusters WHERE name = (.+)$").
				WithArgs(clusterName).
				WillReturnRows(sqlmock.NewRows([]string{"yaml"}).AddRow(yaml1))

			_, err := models.NewCluster(app.DB, "user", clusterName, &mTest.MockReadiness{}, &mTest.MockReadiness{}, config)
			Expect(err).NotTo(HaveOccurred())

			ctx := NewContextWithEmail(request.Context(), "user@example.com")
			clusterHandler.ServeHTTP(recorder, request.WithContext(ctx))

			Expect(recorder.Header().Get("Content-Type")).To(Equal("application/json"))
			bodyJSON := make(map[string]string)
			json.Unmarshal(recorder.Body.Bytes(), &bodyJSON)
			Expect(bodyJSON["description"]).To(Equal("namespace for user 'user' not found"))
			Expect(bodyJSON["error"]).To(Equal("get apps error"))
			Expect(bodyJSON["code"]).To(Equal("MST-004"))
			Expect(recorder.Code).To(Equal(http.StatusNotFound))
		})
	})
})
