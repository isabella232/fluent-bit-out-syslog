package syslog_test

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf/fluent-bit-out-syslog/pkg/syslog"
)

var _ = Describe("Out", func() {
	Context("Insecure TCP", func() {
		It("filters messages based on namespace_name in kubernetes metadata", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()

			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "kube-system",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			record := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"pod_name":       []byte("etcd-minikube"),
					"namespace_name": []byte("kube-system"),
					"host":           []byte("some-host"),
					"container_name": []byte("etcd"),
				},
			}

			err := out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceived(
				"92 <14>1 1970-01-01T00:00:00+00:00 some-host kube-system/pod/etcd-minikube/etcd - - - some-log\n",
			)
		})

		It("drops messages with unconfigured namespaces", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()

			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "test-namespace",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			r1 := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"pod_name":       []byte("etcd-minikube"),
					"namespace_name": []byte("kube-system"),
					"host":           []byte("some-host"),
					"container_name": []byte("etcd"),
				},
			}
			r2 := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"pod_name":       []byte("etcd-minikube"),
					"namespace_name": []byte("test-namespace"),
					"host":           []byte("some-host"),
					"container_name": []byte("etcd"),
				},
			}

			err := out.Write(r1, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())
			err = out.Write(r2, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceivedOnly(
				"95 <14>1 1970-01-01T00:00:00+00:00 some-host test-namespace/pod/etcd-minikube/etcd - - - some-log\n",
			)
		})

		It("truncates the app name if there is too much information", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()

			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "namespace-name-very-long",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			record := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"pod_name":       []byte("pod-name"),
					"namespace_name": []byte("namespace-name-very-long"),
					"host":           []byte("some-host"),
					"container_name": []byte("container-name-very-long"),
				},
			}

			err := out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceived(
				"106 <14>1 1970-01-01T00:00:00+00:00 some-host namespace-name-very-long/pod/pod-name/container- - - - some-log\n",
			)
		})

		It("doesn't add a newline if one already exists in the message", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()

			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "namespace-name-very-long",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			record := map[interface{}]interface{}{
				"log": []byte("some-log\n"),
				"kubernetes": map[interface{}]interface{}{
					"pod_name":       []byte("pod-name"),
					"namespace_name": []byte("namespace-name-very-long"),
					"host":           []byte("some-host"),
					"container_name": []byte("container-name-very-long"),
				},
			}

			err := out.Write(record, time.Unix(0, 0).UTC(), "")

			Expect(err).ToNot(HaveOccurred())
			spyDrain.expectReceivedOnly(
				"106 <14>1 1970-01-01T00:00:00+00:00 some-host namespace-name-very-long/pod/pod-name/container- - - - some-log\n",
			)
		})

		It("returns an error when unable to write the message", func() {
			spyDrain := newSpyDrain()
			s := syslog.Sink{
				Addr: spyDrain.url(),
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			spyDrain.stop()

			err := out.Write(nil, time.Time{}, "")

			Expect(err).To(HaveOccurred())
		})

		It("eventually connects to a failing syslog drain", func() {
			spyDrain := newSpyDrain()
			spyDrain.stop()
			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "some-namespace",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			record := map[interface{}]interface{}{
				"log": []byte("some-log-message"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-namespace"),
				},
			}

			err := out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).To(HaveOccurred())

			spyDrain = newSpyDrain(spyDrain.url())

			err = out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceived(
				"78 <14>1 1970-01-01T00:00:00+00:00 - some-namespace/pod// - - - some-log-message\n",
			)
		})

		It("doesn't reconnect if connection already established", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()
			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "some-namespace",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})

			record := map[interface{}]interface{}{
				"log": []byte("some-log-message"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-namespace"),
				},
			}
			err := out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceived(
				"78 <14>1 1970-01-01T00:00:00+00:00 - some-namespace/pod// - - - some-log-message\n",
			)

			err = out.Write(record, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			done := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(done)
				_, _ = spyDrain.lis.Accept()
			}()
			Consistently(done).ShouldNot(BeClosed())
		})

		It("reconnects if previous connection went away", func() {
			spyDrain := newSpyDrain()
			s := syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "some-namespace",
			}
			out := syslog.NewOut([]*syslog.Sink{&s})
			r1 := map[interface{}]interface{}{
				"log": []byte("some-log-message"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-namespace"),
				},
			}

			err := out.Write(r1, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())
			spyDrain.expectReceived(
				"78 <14>1 1970-01-01T00:00:00+00:00 - some-namespace/pod// - - - some-log-message\n",
			)

			spyDrain.stop()
			spyDrain = newSpyDrain(spyDrain.url())

			r2 := map[interface{}]interface{}{
				"log": []byte("some-log-message-2"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-namespace"),
				},
			}

			f := func() error {
				return out.Write(r2, time.Unix(0, 0).UTC(), "")
			}
			Eventually(f).Should(HaveOccurred())

			err = out.Write(r2, time.Unix(0, 0).UTC(), "")
			Expect(err).ToNot(HaveOccurred())

			spyDrain.expectReceived(
				"80 <14>1 1970-01-01T00:00:00+00:00 - some-namespace/pod// - - - some-log-message-2\n",
			)
		})

		DescribeTable(
			"sends no data when record",
			func(record map[interface{}]interface{}, message string) {
				spyDrain := newSpyDrain()
				defer spyDrain.stop()
				s := syslog.Sink{
					Addr:      spyDrain.url(),
					Namespace: "some-ns",
				}
				out := syslog.NewOut([]*syslog.Sink{&s})

				err := out.Write(record, time.Unix(0, 0).UTC(), "")
				Expect(err).ToNot(HaveOccurred())

				done := make(chan struct{})
				cleanup := make(chan struct{})
				defer close(cleanup)
				go func() {
					select {
					case <-cleanup:
						return
					default:
						spyDrain.lis.Accept()
						close(done)
					}
				}()

				Consistently(done).ShouldNot(BeClosed())
			},
			Entry(
				"has no k8s map",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
				},
				"",
			),
			Entry(
				"has k8s map of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[string][]byte{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"",
			),
			Entry(
				"has no namespace name",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"86 <14>1 1970-01-01T00:00:00+00:00 some-host /pod/some-pod/some-container - - - some-log\n",
			),
			Entry(
				"has namespace of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []int{1, 2, 3, 4},
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"86 <14>1 1970-01-01T00:00:00+00:00 some-host /pod/some-pod/some-container - - - some-log\n",
			),
		)

		DescribeTable(
			"missing data",
			func(record map[interface{}]interface{}, message string) {
				spyDrain := newSpyDrain()
				defer spyDrain.stop()
				s := syslog.Sink{
					Addr:      spyDrain.url(),
					Namespace: "some-ns",
				}
				out := syslog.NewOut([]*syslog.Sink{&s})

				err := out.Write(record, time.Unix(0, 0).UTC(), "")
				Expect(err).ToNot(HaveOccurred())

				spyDrain.expectReceived(message)
			},
			Entry(
				"no log message",
				map[interface{}]interface{}{
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod/some-pod/some-container - - - \n",
			),
			Entry(
				"log message is of different type",
				map[interface{}]interface{}{
					"log": []int{1, 2, 3, 4},
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod/some-pod/some-container - - - \n",
			),
			Entry(
				"log message key is of different type",
				map[interface{}]interface{}{
					5: []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod/some-pod/some-container - - - \n",
			),
			Entry(
				"no host",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 - some-ns/pod/some-pod/some-container - - - some-log\n",
			),
			Entry(
				"host key is of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						1:                []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 - some-ns/pod/some-pod/some-container - - - some-log\n",
			),
			Entry(
				"host is of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []int{1, 2, 3, 4},
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 - some-ns/pod/some-pod/some-container - - - some-log\n",
			),
			Entry(
				"no container name",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
					},
				},
				"79 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod/some-pod/ - - - some-log\n",
			),
			Entry(
				"container name is of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []byte("some-pod"),
						"container_name": []int{1, 2, 3, 4},
					},
				},
				"79 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod/some-pod/ - - - some-log\n",
			),
			Entry(
				"no pod name",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod//some-container - - - some-log\n",
			),
			Entry(
				"pod name is of different type",
				map[interface{}]interface{}{
					"log": []byte("some-log"),
					"kubernetes": map[interface{}]interface{}{
						"host":           []byte("some-host"),
						"namespace_name": []byte("some-ns"),
						"pod_name":       []int{1, 2, 3, 4},
						"container_name": []byte("some-container"),
					},
				},
				"85 <14>1 1970-01-01T00:00:00+00:00 some-host some-ns/pod//some-container - - - some-log\n",
			),
		)
	})

	Context("Secure with TLS", func() {
		It("writes messages via syslog-tls", func() {
			spyDrain := newTLSSpyDrain()
			defer spyDrain.stop()

			s := &syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "some-ns",
				TLS: &syslog.TLS{
					InsecureSkipVerify: true,
					Timeout:            time.Second,
				},
			}

			out := syslog.NewOut([]*syslog.Sink{s})
			r := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-ns"),
				},
			}

			// TLS will block on waiting for handshake so the write needs
			// to occur in a separate go routine
			go func() {
				defer GinkgoRecover()
				err := out.Write(r, time.Unix(0, 0).UTC(), "")
				Expect(err).ToNot(HaveOccurred())
			}()

			spyDrain.expectReceivedOnly(
				"63 <14>1 1970-01-01T00:00:00+00:00 - some-ns/pod// - - - some-log\n",
			)
		})

		It("fails when connecting to non TLS endpoint", func() {
			spyDrain := newSpyDrain()
			defer spyDrain.stop()

			s := &syslog.Sink{
				Addr:      spyDrain.url(),
				Namespace: "some-ns",
				TLS: &syslog.TLS{
					InsecureSkipVerify: true,
					Timeout:            time.Second,
				},
			}

			out := syslog.NewOut([]*syslog.Sink{s})
			r := map[interface{}]interface{}{
				"log": []byte("some-log"),
				"kubernetes": map[interface{}]interface{}{
					"namespace_name": []byte("some-ns"),
				},
			}

			err := out.Write(r, time.Unix(0, 0).UTC(), "")
			Expect(err).To(HaveOccurred())
		})
	})
})

// This is an example of a message that is sent to the syslog output plugin.
// It is here for documentation purposes.
var _ = map[interface{}]interface{}{
	"log":    []byte("log data"),
	"stream": []byte("stdout"),
	"time":   []byte("2018-07-16T17:47:16.61514406Z"),
	"kubernetes": map[interface{}]interface{}{
		"labels": map[interface{}]interface{}{
			"component":                     []byte("kube-addon-manager"),
			"kubernetes.io/minikube-addons": []byte("addon-manager"),
			"version":                       []byte("v8.6"),
		},
		"annotations": map[interface{}]interface{}{
			"kubernetes.io/config.hash":   []byte{},
			"kubernetes.io/config.mirror": []byte{},
			"kubernetes.io/config.seen":   []byte{},
			"kubernetes.io/config.source": []byte{},
		},
		"host":           []byte("minikube"),
		"container_name": []byte("kube-addon-manager"),
		"docker_id":      []byte("some-hash"),
		"pod_name":       []byte("kube-addon-manager-minikube"),
		"namespace_name": []byte("kube-system"),
		"pod_id":         []byte("some-hash"),
	},
}
