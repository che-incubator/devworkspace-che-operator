package solver

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/che-incubator/devworkspace-che-operator/apis/che-controller/v1alpha1"
	"github.com/che-incubator/devworkspace-che-operator/pkg/defaults"
	"github.com/che-incubator/devworkspace-che-operator/pkg/manager"
	dw "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/api/v2/pkg/attributes"
	dwo "github.com/devfile/devworkspace-operator/apis/controller/v1alpha1"
	"github.com/devfile/devworkspace-operator/controllers/controller/devworkspacerouting/solvers"
	"github.com/devfile/devworkspace-operator/pkg/constants"
	"github.com/devfile/devworkspace-operator/pkg/infrastructure"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensions "k8s.io/api/extensions/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	apiext "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"
)

func createTestScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(extensions.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))
	utilruntime.Must(rbac.AddToScheme(scheme))
	utilruntime.Must(dw.AddToScheme(scheme))
	utilruntime.Must(dwo.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))

	return scheme
}

func getSpecObjectsForManager(t *testing.T, mgr *v1alpha1.CheManager, routing *dwo.DevWorkspaceRouting, additionalInitialObjects ...runtime.Object) (client.Client, solvers.RoutingSolver, solvers.RoutingObjects) {
	scheme := createTestScheme()

	allObjs := []runtime.Object{mgr}
	for i := range additionalInitialObjects {
		allObjs = append(allObjs, additionalInitialObjects[i])
	}
	cl := fake.NewFakeClientWithScheme(scheme, allObjs...)

	solver, err := Getter(scheme).GetSolver(cl, "che")
	if err != nil {
		t.Fatal(err)
	}

	meta := solvers.DevWorkspaceMetadata{
		DevWorkspaceId: routing.Spec.DevWorkspaceId,
		Namespace:      routing.GetNamespace(),
		PodSelector:    routing.Spec.PodSelector,
		RoutingSuffix:  routing.Spec.RoutingSuffix,
	}

	// we need to do 1 round of che manager reconciliation so that the solver gets initialized
	cheRecon := manager.New(cl, scheme)
	_, err = cheRecon.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: mgr.Name, Namespace: mgr.Namespace}})
	if err != nil {
		t.Fatal(err)
	}

	objs, err := solver.GetSpecObjects(routing, meta)
	if err != nil {
		t.Fatal(err)
	}

	// now we need a second round of che manager reconciliation so that it proclaims the che gateway as established
	cheRecon.Reconcile(reconcile.Request{NamespacedName: types.NamespacedName{Name: "che", Namespace: "ns"}})

	return cl, solver, objs
}

func getSpecObjects(t *testing.T, routing *dwo.DevWorkspaceRouting) (client.Client, solvers.RoutingSolver, solvers.RoutingObjects) {
	return getSpecObjectsForManager(t, &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{manager.FinalizerName},
		},
		Spec: v1alpha1.CheManagerSpec{
			GatewayHost: "over.the.rainbow",
		},
	}, routing)
}

func subdomainDevWorkspaceRouting() *dwo.DevWorkspaceRouting {
	return &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			RoutingSuffix:  "over.the.rainbow",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
					},
					{
						Name:       "e2",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "http",
						Path:       "/2.js",
						Secure:     true,
					},
					{
						Name:       "e3",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
					},
				},
			},
		},
	}
}

func relocatableDevWorkspaceRouting() *dwo.DevWorkspaceRouting {
	return &dwo.DevWorkspaceRouting{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "routing",
			Namespace: "ws",
		},
		Spec: dwo.DevWorkspaceRoutingSpec{
			DevWorkspaceId: "wsid",
			RoutingClass:   "che",
			RoutingSuffix:  "over.the.rainbow",
			Endpoints: map[string]dwo.EndpointList{
				"m1": {
					{
						Name:       "e1",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "https",
						Path:       "/1/",
						Attributes: attributes.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
					{
						Name:       "e2",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Protocol:   "http",
						Path:       "/2.js",
						Secure:     true,
						Attributes: attributes.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
					{
						Name:       "e3",
						TargetPort: 9999,
						Exposure:   dw.PublicEndpointExposure,
						Attributes: attributes.Attributes{
							urlRewriteSupportedEndpointAttributeName: apiext.JSON{Raw: []byte("\"true\"")},
						},
					},
				},
			},
		},
	}
}

func TestCreateRelocatedObjects(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	cl, _, objs := getSpecObjects(t, relocatableDevWorkspaceRouting())

	t.Run("noIngresses", func(t *testing.T) {
		if len(objs.Ingresses) != 0 {
			t.Error()
		}
	})

	t.Run("noRoutes", func(t *testing.T) {
		if len(objs.Routes) != 0 {
			t.Error()
		}
	})

	t.Run("noPodAdditions", func(t *testing.T) {
		if objs.PodAdditions != nil {
			t.Error()
		}
	})

	for i := range objs.Services {
		t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
			svc := &objs.Services[i]
			if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
				t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
				t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
			}

			if svc.Labels[constants.DevWorkspaceIDLabel] != "wsid" {
				t.Errorf("The workspace ID should be recorded in the service labels")
			}
		})
	}

	t.Run("traefikConfig", func(t *testing.T) {
		cms := &corev1.ConfigMapList{}
		cl.List(context.TODO(), cms)

		if len(cms.Items) != 2 {
			t.Errorf("there should be 2 configmaps created for the gateway config of the workspace and che but there were: %d", len(cms.Items))
		}

		var cheMgrCfg *corev1.ConfigMap
		var workspaceCfg *corev1.ConfigMap

		for _, cfg := range cms.Items {
			if cfg.Name == "che" {
				cheMgrCfg = &cfg
			}

			if cfg.Name == "wsid" {
				workspaceCfg = &cfg
			}
		}

		if cheMgrCfg == nil {
			t.Error("traefik configuration for che manager not found")
		}

		if workspaceCfg == nil {
			t.Fatalf("traefik configuration for the workspace not found")
		}

		traefikWorkspaceConfig := workspaceCfg.Data["wsid.yml"]

		if len(traefikWorkspaceConfig) == 0 {
			t.Fatal("No traefik config file found in the workspace config configmap")
		}

		workspaceConfig := traefikConfig{}
		if err := yaml.Unmarshal([]byte(traefikWorkspaceConfig), &workspaceConfig); err != nil {
			t.Fatal(err)
		}

		if len(workspaceConfig.HTTP.Routers) != 1 {
			t.Fatalf("Expected exactly one traefik router but got %d", len(workspaceConfig.HTTP.Routers))
		}

		if _, ok := workspaceConfig.HTTP.Routers["wsid-m1-9999"]; !ok {
			t.Fatal("traefik config doesn't contain expected workspace configuration")
		}
	})
}

func TestCreateSubDomainObjects(t *testing.T) {
	testCommon := func(infra infrastructure.Type) solvers.RoutingObjects {
		infrastructure.InitializeForTesting(infra)

		cl, _, objs := getSpecObjects(t, subdomainDevWorkspaceRouting())

		t.Run("noPodAdditions", func(t *testing.T) {
			if objs.PodAdditions != nil {
				t.Error()
			}
		})

		for i := range objs.Services {
			t.Run(fmt.Sprintf("service-%d", i), func(t *testing.T) {
				svc := &objs.Services[i]
				if svc.Annotations[defaults.ConfigAnnotationCheManagerName] != "che" {
					t.Errorf("The name of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Annotations[defaults.ConfigAnnotationCheManagerNamespace] != "ns" {
					t.Errorf("The namespace of the associated che manager should have been recorded in the service annotation")
				}

				if svc.Labels[constants.DevWorkspaceIDLabel] != "wsid" {
					t.Errorf("The workspace ID should be recorded in the service labels")
				}
			})
		}

		t.Run("noWorkspaceTraefikConfig", func(t *testing.T) {
			cms := &corev1.ConfigMapList{}
			cl.List(context.TODO(), cms)

			if len(cms.Items) != 1 {
				t.Errorf("there should be 1 configmaps created for the gateway config of the workspace and che but there were: %d", len(cms.Items))
			}

			cm := cms.Items[0]
			if _, ok := cm.Data["traefik.yml"]; !ok {
				t.Errorf("There should be basic gateway configuration with a config file called `traefik.yml`, but none such found.")
			}
		})

		return objs
	}

	t.Run("expectedIngresses", func(t *testing.T) {
		objs := testCommon(infrastructure.Kubernetes)
		if len(objs.Ingresses) != 1 {
			t.Error()
		}
	})

	t.Run("expectedRoutes", func(t *testing.T) {
		objs := testCommon(infrastructure.OpenShiftv4)
		if len(objs.Routes) != 1 {
			t.Error()
		}
	})
}

func TestReportRelocatableExposedEndpoints(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := relocatableDevWorkspaceRouting()
	_, solver, objs := getSpecObjects(t, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://over.the.rainbow/wsid/m1/9999/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://over.the.rainbow/wsid/m1/9999/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "https://over.the.rainbow/wsid/m1/9999/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://over.the.rainbow/wsid/m1/9999/", e3.Url)
	}
}

func TestReportSubdomainExposedEndpoints(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := subdomainDevWorkspaceRouting()
	_, solver, objs := getSpecObjects(t, routing)

	exposed, ready, err := solver.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should have been ready.")
	}

	if len(exposed) != 1 {
		t.Errorf("There should have been 1 exposed endpoins but found %d", len(exposed))
	}

	m1, ok := exposed["m1"]
	if !ok {
		t.Errorf("The exposed endpoints should have been defined on the m1 component.")
	}

	if len(m1) != 3 {
		t.Fatalf("There should have been 3 endpoints for m1.")
	}

	e1 := m1[0]
	if e1.Name != "e1" {
		t.Errorf("The first endpoint should have been e1 but is %s", e1.Name)
	}
	if e1.Url != "https://wsid-1.over.the.rainbow/1/" {
		t.Errorf("The e1 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-1.over.the.rainbow/1/", e1.Url)
	}

	e2 := m1[1]
	if e2.Name != "e2" {
		t.Errorf("The second endpoint should have been e2 but is %s", e1.Name)
	}
	if e2.Url != "https://wsid-1.over.the.rainbow/2.js" {
		t.Errorf("The e2 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-1.over.the.rainbow/2.js", e2.Url)
	}

	e3 := m1[2]
	if e3.Name != "e3" {
		t.Errorf("The third endpoint should have been e3 but is %s", e1.Name)
	}
	if e3.Url != "http://wsid-1.over.the.rainbow/" {
		t.Errorf("The e3 endpoint should have the following URL: '%s' but has '%s'.", "https://wsid-1.over.the.rainbow/", e3.Url)
	}
}

func TestFinalize(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := relocatableDevWorkspaceRouting()
	cl, slv, _ := getSpecObjects(t, routing)

	// the create test checks that during the above call, the solver created the 2 traefik configmaps
	// (1 for the main config and the second for the devworkspace)

	// now, let the solver finalize the routing
	if err := slv.Finalize(routing); err != nil {
		t.Fatal(err)
	}

	cms := &corev1.ConfigMapList{}
	cl.List(context.TODO(), cms)

	if len(cms.Items) != 1 {
		t.Fatalf("There should be just 1 configmap after routing finalization, but there were %d found", len(cms.Items))
	}

	cm := cms.Items[0]
	if cm.Name != "che" {
		t.Fatal("The only configmap left should be the main traefik config, but the configmap has unexpected name")
	}
}

func TestEndpointsAlwaysOnSecureProtocolsWhenExposedThroughGateway(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)
	routing := relocatableDevWorkspaceRouting()
	_, slv, objs := getSpecObjects(t, routing)

	exposed, ready, err := slv.GetExposedEndpoints(routing.Spec.Endpoints, objs)
	if err != nil {
		t.Fatal(err)
	}

	if !ready {
		t.Errorf("The exposed endpoints should be considered ready.")
	}

	for _, endpoints := range exposed {
		for _, endpoint := range endpoints {
			if !strings.HasPrefix(endpoint.Url, "https://") {
				t.Errorf("The endpoint %s should be exposed on https.", endpoint.Url)
			}
		}
	}
}

func TestUsesIngressAnnotationsForWorkspaceEndpointIngresses(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	mgr := &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{manager.FinalizerName},
		},
		Spec: v1alpha1.CheManagerSpec{
			GatewayHost: "over.the.rainbow",
			K8s: v1alpha1.CheManagerSpecK8s{
				IngressAnnotations: map[string]string{
					"a": "b",
				},
			},
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting())

	if len(objs.Ingresses) != 1 {
		t.Fatalf("Unexpected number of generated ingresses: %d", len(objs.Ingresses))
	}

	ingress := objs.Ingresses[0]
	if len(ingress.Annotations) != 3 {
		// 3 annotations - a => b, endpoint-name and component-name
		t.Fatalf("Unexpected number of annotations on the generated ingress: %d", len(ingress.Annotations))
	}

	if ingress.Annotations["a"] != "b" {
		t.Errorf("Unexpected value of the custom endpoint ingress annotation")
	}
}

func TestUsesCustomCertificateForWorkspaceEndpointIngresses(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.Kubernetes)

	mgr := &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{manager.FinalizerName},
		},
		Spec: v1alpha1.CheManagerSpec{
			GatewayHost:   "beyond.comprehension",
			TlsSecretName: "tlsSecret",
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tlsSecret",
			Namespace: "ns",
		},
		Data: map[string][]byte{
			"tls.key": []byte("asdf"),
			"tls.crt": []byte("qwer"),
		},
	})

	if len(objs.Ingresses) != 1 {
		t.Fatalf("Unexpected number of generated ingresses: %d", len(objs.Ingresses))
	}

	ingress := objs.Ingresses[0]

	if len(ingress.Spec.TLS) != 1 {
		t.Fatalf("Unexpected number of TLS records on the ingress: %d", len(ingress.Spec.TLS))
	}

	if ingress.Spec.TLS[0].SecretName != "wsid-endpoints" {
		t.Errorf("Unexpected name of the TLS secret on the ingress: %s", ingress.Spec.TLS[0].SecretName)
	}

	if len(ingress.Spec.TLS[0].Hosts) != 1 {
		t.Fatalf("Unexpected number of host records on the TLS spec: %d", len(ingress.Spec.TLS[0].Hosts))
	}

	if ingress.Spec.TLS[0].Hosts[0] != "wsid-1.over.the.rainbow" {
		t.Errorf("Unexpected host name of the TLS spec: %s", ingress.Spec.TLS[0].Hosts[0])
	}
}

func TestUsesCustomCertificateForWorkspaceEndpointRoutes(t *testing.T) {
	infrastructure.InitializeForTesting(infrastructure.OpenShiftv4)

	mgr := &v1alpha1.CheManager{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "che",
			Namespace:  "ns",
			Finalizers: []string{manager.FinalizerName},
		},
		Spec: v1alpha1.CheManagerSpec{
			GatewayHost:   "beyond.comprehension",
			TlsSecretName: "tlsSecret",
		},
	}

	_, _, objs := getSpecObjectsForManager(t, mgr, subdomainDevWorkspaceRouting(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tlsSecret",
			Namespace: "ns",
		},
		Data: map[string][]byte{
			"tls.key": []byte("asdf"),
			"tls.crt": []byte("qwer"),
		},
	})

	if len(objs.Routes) != 1 {
		t.Fatalf("Unexpected number of generated routes: %d", len(objs.Routes))
	}

	route := objs.Routes[0]

	if route.Spec.TLS.Certificate != "qwer" {
		t.Errorf("Unexpected name of the TLS certificate on the route: %s", route.Spec.TLS.Certificate)
	}

	if route.Spec.TLS.Key != "asdf" {
		t.Errorf("Unexpected key of TLS spec: %s", route.Spec.TLS.Key)
	}
}
