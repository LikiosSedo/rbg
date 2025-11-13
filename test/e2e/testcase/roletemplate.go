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
								Image: "nginx:latest",
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

					gomega.Expect(sts.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("nginx:latest"))

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
								Image: "nginx:1.19",
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
								Image: "nginx:1.20",
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
					}, 30*time.Second, 1*time.Second).Should(gomega.Equal("nginx:1.20"))

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
								Image: "nginx:1.19",
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
								Image: "nginx:templated",
							},
						}).Obj()

					emptyPatch := buildTemplatePatch(map[string]interface{}{})

					traditionalTemplate := wrappers.BuildBasicPodTemplateSpec().
						WithContainers([]corev1.Container{
							{
								Name:  "app",
								Image: "nginx:traditional",
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
					gomega.Expect(sts1.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("nginx:templated"))

					// Verify traditional role
					sts2 := &appsv1.StatefulSet{}
					gomega.Eventually(func() error {
						return f.Client.Get(f.Ctx, types.NamespacedName{
							Name:      fmt.Sprintf("%s-%s", rbg.Name, "traditional-role"),
							Namespace: f.Namespace,
						}, sts2)
					}, 30*time.Second, 1*time.Second).Should(gomega.Succeed())
					gomega.Expect(sts2.Spec.Template.Spec.Containers[0].Image).To(gomega.Equal("nginx:traditional"))

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
