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

	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/pkg/fields"
	"k8s.io/client-go/pkg/labels"
)

var _ = Describe("Service", func() {
	var (
		clientset *fake.Clientset
		name      = "test"
		namespace = "mystack-user"
		username  = "user"
		portMaps  = []*PortMap{
			&PortMap{Port: 80, TargetPort: 5000},
		}
		labelMap    = labels.Set{"mystack/routable": "true"}
		listOptions = v1.ListOptions{
			LabelSelector: labelMap.AsSelector().String(),
			FieldSelector: fields.Everything().String(),
		}
	)

	BeforeEach(func() {
		clientset = fake.NewSimpleClientset()
	})

	Describe("Expose", func() {
		It("should expose a new Service", func() {
			service := NewService(name, username, portMaps, false)
			Expect(service.Namespace).To(Equal(namespace))

			servicev1, err := service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(servicev1.GetNamespace()).To(Equal(namespace))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(1))

			Expect(servicev1.GetLabels()["mystack/service"]).To(Equal("false"))
		})

		It("should expose a new Service that is not mystack service", func() {
			service := NewService(name, username, portMaps, true)
			Expect(service.Namespace).To(Equal(namespace))

			servicev1, err := service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())
			Expect(servicev1.GetNamespace()).To(Equal(namespace))

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(1))

			Expect(servicev1.GetLabels()["mystack/service"]).To(Equal("true"))
		})

		It("should return error when creating same service twice", func() {
			service := NewService(name, username, portMaps, false)
			Expect(service.Namespace).To(Equal(namespace))

			_, err := service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())

			_, err = service.Expose(clientset)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal("Service \"test\" already exists"))
			Expect(fmt.Sprintf("%T", err)).To(Equal("*errors.KubernetesError"))
		})
	})

	Describe("Delete", func() {
		It("should return error if trying to delete unexposed service", func() {
			service := NewService(name, username, portMaps, false)
			err := service.Delete(clientset)
			Expect(err).To(HaveOccurred())
		})

		It("should delete service", func() {
			service := NewService(name, username, portMaps, false)
			_, err := service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = service.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(0))
		})

		It("should not delete all services", func() {
			service := NewService(name, username, portMaps, false)
			_, err := service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())

			service2 := NewService("test2", username, portMaps, false)
			_, err = service2.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())

			err = service.Delete(clientset)
			Expect(err).NotTo(HaveOccurred())

			services, err := clientset.CoreV1().Services(namespace).List(listOptions)
			Expect(err).NotTo(HaveOccurred())
			Expect(services.Items).To(HaveLen(1))
		})
	})

	Describe("ServicePort", func() {
		It("should return error if namespace doesn't exist", func() {
			_, err := ServicePort(clientset, name, username)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`Service "test" not found`))
		})

		It("should return error if service doesn't exist", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			_, err = ServicePort(clientset, name, username)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Equal(`Service "test" not found`))
		})

		It("should return correct port", func() {
			err := CreateNamespace(clientset, username)
			Expect(err).NotTo(HaveOccurred())

			service := NewService(name, username, portMaps, true)
			_, err = service.Expose(clientset)
			Expect(err).NotTo(HaveOccurred())

			port, err := ServicePort(clientset, name, username)
			Expect(err).NotTo(HaveOccurred())
			Expect(port).To(Equal(80))
		})
	})
})
