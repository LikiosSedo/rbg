package testcase

import (
	"fmt"
	"time"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	workloadsv1alpha1 "sigs.k8s.io/rbgs/api/workloads/v1alpha1"
	"sigs.k8s.io/rbgs/pkg/utils"
	"sigs.k8s.io/rbgs/test/e2e/framework"
	"sigs.k8s.io/rbgs/test/wrappers"
)

func RunRoleTemplateTestCases(f *framework.Framework) {
	ginkgo.Describe(
		"roletemplate controller", func() {

			ginkgo.It(
				"create rbg with roleTemplates and verify workloads", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "nginx",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						}).Obj()

					role1Patch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "nginx",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu": "200m",
										},
									},
								},
							},
						},
					})

					role2Patch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "nginx",
									// Keep nginx:latest, verify patch by checking resources
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":    "150m",
											"memory": "256Mi",
										},
									},
								},
							},
						},
					})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-roletemplate", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{
								Name:     "base-template",
								Template: baseTemplate,
							},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base-template").
								WithTemplatePatch(role1Patch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
							wrappers.BuildBasicRole("role2").
								WithTemplateRef("base-template").
								WithTemplatePatch(role2Patch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					// Verify role1 StatefulSet
					sts1 := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts1)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Expect(sts1.Spec.Template.Spec.Containers).To(gomega.HaveLen(1))
					container1 := sts1.Spec.Template.Spec.Containers[0]
					gomega.Expect(container1.Image).To(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))
					gomega.Expect(container1.Resources.Requests[corev1.ResourceCPU]).To(gomega.Equal(resource.MustParse("200m")))

					// Verify role2 StatefulSet
					sts2 := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role2"),
							Namespace: f.Namespace,
						}, sts2)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					container2 := sts2.Spec.Template.Spec.Containers[0]
					gomega.Expect(container2.Image).To(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))
					gomega.Expect(container2.Resources.Requests[corev1.ResourceCPU]).To(gomega.Equal(resource.MustParse("150m")))
					gomega.Expect(container2.Resources.Requests[corev1.ResourceMemory]).To(gomega.Equal(resource.MustParse("256Mi")))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"create rbg with empty templatePatch", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-empty-patch", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					sts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"update roleTemplate and trigger rollout", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-update-template", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								WithReplicas(2).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					sts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					initialRevision := sts.Status.CurrentRevision

					// Update roleTemplate
					updatedTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Env: []corev1.EnvVar{
									{Name: "VERSION", Value: "v2"},
								},
							},
						}).Obj()

					gomega.Eventually(func() error {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      rbg.Name,
							Namespace: f.Namespace,
						}, rbg)
						if err != nil {
							return err
						}
						rbg.Spec.RoleTemplates[0].Template = updatedTemplate
						return f.Client.Update(f.Ctx, rbg)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					// Verify rollout happens
					gomega.Eventually(func() bool {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
						if err != nil {
							return false
						}
						return sts.Status.CurrentRevision != initialRevision && sts.Status.UpdateRevision != ""
					}, 60*time.Second, 2*time.Second).Should(gomega.BeTrue())

					gomega.Eventually(func() string {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
						if err != nil {
							return ""
						}
						return sts.Spec.Template.Spec.Containers[0].Image
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"update templatePatch without changing roleTemplate triggers update", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						}).Obj()

					initialPatch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "app",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu": "200m",
										},
									},
								},
							},
						},
					})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-patch-update", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base").
								WithTemplatePatch(initialPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								WithReplicas(1).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					sts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					initialCPU := sts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
					gomega.Expect(initialCPU).To(gomega.Equal(resource.MustParse("200m")))

					initialRevision := sts.Status.CurrentRevision

					updatedPatch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "app",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu": "300m",
										},
									},
								},
							},
						},
					})

					gomega.Eventually(func() error {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      rbg.Name,
							Namespace: f.Namespace,
						}, rbg)
						if err != nil {
							return err
						}
						rbg.Spec.Roles[0].TemplatePatch = updatedPatch
						return f.Client.Update(f.Ctx, rbg)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Eventually(func() bool {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
						if err != nil {
							return false
						}
						return sts.Status.UpdateRevision != "" &&
							sts.Status.UpdateRevision != initialRevision
					}, 60*time.Second, 2*time.Second).Should(gomega.BeTrue())

					gomega.Eventually(func() string {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "role1"),
							Namespace: f.Namespace,
						}, sts)
						if err != nil {
							return ""
						}
						cpu := sts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
						return cpu.String()
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("300m"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"multiple roles share same roleTemplate with different patches", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
							},
						}).Obj()

					workerPatch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "app",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu": "500m",
										},
									},
								},
							},
						},
					})

					cachePatch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "app",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"memory": "512Mi",
										},
									},
								},
							},
						},
					})

					proxyPatch := buildTemplatePatch(map[string]interface{}{
						"spec": map[string]interface{}{
							"containers": []map[string]interface{}{
								{
									"name": "app",
									"resources": map[string]interface{}{
										"requests": map[string]interface{}{
											"cpu":    "50m",
											"memory": "64Mi",
										},
									},
								},
							},
						},
					})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-shared-template", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "common-app", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("worker").
								WithTemplateRef("common-app").
								WithTemplatePatch(workerPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
							wrappers.BuildBasicRole("cache").
								WithTemplateRef("common-app").
								WithTemplatePatch(cachePatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
							wrappers.BuildBasicRole("proxy").
								WithTemplateRef("common-app").
								WithTemplatePatch(proxyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					workerSts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "worker"),
							Namespace: f.Namespace,
						}, workerSts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					workerCPU := workerSts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
					gomega.Expect(workerCPU).To(gomega.Equal(resource.MustParse("500m")))
					gomega.Expect(workerSts.Spec.Template.Spec.Containers[0].Image).To(
						gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					cacheSts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "cache"),
							Namespace: f.Namespace,
						}, cacheSts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					cacheMem := cacheSts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
					gomega.Expect(cacheMem).To(gomega.Equal(resource.MustParse("512Mi")))
					gomega.Expect(cacheSts.Spec.Template.Spec.Containers[0].Image).To(
						gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					proxySts := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "proxy"),
							Namespace: f.Namespace,
						}, proxySts)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					proxyCPU := proxySts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceCPU]
					proxyMem := proxySts.Spec.Template.Spec.Containers[0].Resources.Requests[corev1.ResourceMemory]
					gomega.Expect(proxyCPU).To(gomega.Equal(resource.MustParse("50m")))
					gomega.Expect(proxyMem).To(gomega.Equal(resource.MustParse("64Mi")))
					gomega.Expect(proxySts.Spec.Template.Spec.Containers[0].Image).To(
						gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"update shared roleTemplate triggers all dependent roles to update", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Env: []corev1.EnvVar{
									{Name: "VERSION", Value: "v1"},
								},
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-shared-update", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								WithReplicas(1).
								Obj(),
							wrappers.BuildBasicRole("role2").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								WithReplicas(1).
								Obj(),
							wrappers.BuildBasicRole("role3").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								WithReplicas(1).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					sts1 := &appsv1.StatefulSet{}
					sts2 := &appsv1.StatefulSet{}
					sts3 := &appsv1.StatefulSet{}

					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role1", rbg.Name), Namespace: f.Namespace,
						}, sts1)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role2", rbg.Name), Namespace: f.Namespace,
						}, sts2)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role3", rbg.Name), Namespace: f.Namespace,
						}, sts3)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role1", rbg.Name), Namespace: f.Namespace,
						}, sts1)
						if len(sts1.Spec.Template.Spec.Containers) > 0 && len(sts1.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts1.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 10*time.Second, 1*time.Second).Should(gomega.Equal("v1"))

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role2", rbg.Name), Namespace: f.Namespace,
						}, sts2)
						if len(sts2.Spec.Template.Spec.Containers) > 0 && len(sts2.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts2.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 10*time.Second, 1*time.Second).Should(gomega.Equal("v1"))

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role3", rbg.Name), Namespace: f.Namespace,
						}, sts3)
						if len(sts3.Spec.Template.Spec.Containers) > 0 && len(sts3.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts3.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 10*time.Second, 1*time.Second).Should(gomega.Equal("v1"))

					initialRevision1 := sts1.Status.CurrentRevision
					initialRevision2 := sts2.Status.CurrentRevision
					initialRevision3 := sts3.Status.CurrentRevision

					updatedTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("128Mi"),
									},
								},
								Env: []corev1.EnvVar{
									{Name: "VERSION", Value: "v2"},
								},
							},
						}).Obj()

					gomega.Eventually(func() error {
						err := f.Client.Get(f.Ctx, types.NamespacedName{
							Name: rbg.Name, Namespace: f.Namespace,
						}, rbg)
						if err != nil {
							return err
						}
						rbg.Spec.RoleTemplates[0].Template = updatedTemplate
						return f.Client.Update(f.Ctx, rbg)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())

					gomega.Eventually(func() bool {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role1", rbg.Name), Namespace: f.Namespace,
						}, sts1)
						return sts1.Status.UpdateRevision != "" &&
							sts1.Status.UpdateRevision != initialRevision1
					}, 90*time.Second, 2*time.Second).Should(gomega.BeTrue())

					gomega.Eventually(func() bool {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role2", rbg.Name), Namespace: f.Namespace,
						}, sts2)
						return sts2.Status.UpdateRevision != "" &&
							sts2.Status.UpdateRevision != initialRevision2
					}, 90*time.Second, 2*time.Second).Should(gomega.BeTrue())

					gomega.Eventually(func() bool {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role3", rbg.Name), Namespace: f.Namespace,
						}, sts3)
						return sts3.Status.UpdateRevision != "" &&
							sts3.Status.UpdateRevision != initialRevision3
					}, 90*time.Second, 2*time.Second).Should(gomega.BeTrue())

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role1", rbg.Name), Namespace: f.Namespace,
						}, sts1)
						if len(sts1.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts1.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("v2"))

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role2", rbg.Name), Namespace: f.Namespace,
						}, sts2)
						if len(sts2.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts2.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("v2"))

					gomega.Eventually(func() string {
						f.Client.Get(f.Ctx, types.NamespacedName{
							Name: fmt.Sprintf("%s-role3", rbg.Name), Namespace: f.Namespace,
						}, sts3)
						if len(sts3.Spec.Template.Spec.Containers[0].Env) > 0 {
							return sts3.Spec.Template.Spec.Containers[0].Env[0].Value
						}
						return ""
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("v2"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"verify controllerrevision includes roleTemplates", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-revision-test", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("role1").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)
					f.ExpectRBGRevisionEqual(rbg)

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)

			ginkgo.It(
				"create rbg with mixed mode - some roles use templateRef, others use template", func() {
					baseTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					traditionalTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "registry-cn-shanghai.siflow.cn/k8s/nginx:latest",
							},
						}).Obj()

					rbg := wrappers.BuildBasicRoleBasedGroup("e2e-mixed-mode", f.Namespace).
						WithRoleTemplates([]workloadsv1alpha1.RoleTemplate{
							{Name: "base", Template: baseTemplate},
						}).
						WithRoles([]workloadsv1alpha1.RoleSpec{
							wrappers.BuildBasicRole("templated-role").
								WithTemplateRef("base").
								WithTemplatePatch(emptyPatch).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
							wrappers.BuildBasicRole("traditional-role").
								WithTemplate(traditionalTemplate).
								WithWorkload(workloadsv1alpha1.StatefulSetWorkloadType).
								Obj(),
						}).Obj()

					gomega.Expect(f.Client.Create(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgEqual(rbg)

					// Verify templated role
					sts1 := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "templated-role"),
							Namespace: f.Namespace,
						}, sts1)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())
					gomega.Expect(sts1.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					// Verify traditional role
					sts2 := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "traditional-role"),
							Namespace: f.Namespace,
						}, sts2)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())
					gomega.Expect(sts2.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("registry-cn-shanghai.siflow.cn/k8s/nginx:latest"))

					gomega.Expect(f.Client.Delete(f.Ctx, rbg)).Should(gomega.Succeed())
					f.ExpectRbgDeleted(rbg)
				},
			)
		},
	)
}

func buildTemplatePatch(data map[string]interface{}) runtime.RawExtension {
	return runtime.RawExtension{Raw: []byte(utils.DumpJSON(data))}
}
