package v1alpha1

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func TestValidateRoleTemplates(t *testing.T) {
	tests := []struct {
		name    string
		rbg     *RoleBasedGroup
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid roleTemplate",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "base",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{
										{Name: "app", Image: "nginx"},
									},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate template name",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "base",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
						{
							Name: "base", // duplicate
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "duplicate template name",
		},
		{
			name: "template without containers",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name:     "base",
							Template: corev1.PodTemplateSpec{}, // empty template, no containers
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "must have at least one container",
		},
		{
			name: "invalid template name format (uppercase)",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "Base", // invalid: contains uppercase
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "not a valid DNS label",
		},
		{
			name: "invalid template name format (starts with hyphen)",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "-invalid", // invalid: starts with hyphen
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "not a valid DNS label",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleTemplates(tt.rbg)

			// Check if error matches expectation
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleTemplates() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// If error is expected, check error message contains keyword
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateRoleTemplates() error message = %q, want to contain %q",
						err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestValidateRoleTemplateReferences(t *testing.T) {
	tests := []struct {
		name    string
		rbg     *RoleBasedGroup
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid templateRef",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "base",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
					Roles: []RoleSpec{
						{
							Name:          "prefill",
							Replicas:      ptr.To(int32(1)),
							TemplateRef:   &TemplateRef{Name: "base"},
							TemplatePatch: runtime.RawExtension{Raw: []byte(`{"spec":{}}`)},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "templateRef points to non-existent template",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					Roles: []RoleSpec{
						{
							Name:          "prefill",
							Replicas:      ptr.To(int32(1)),
							TemplateRef:   &TemplateRef{Name: "nonexistent"},
							TemplatePatch: runtime.RawExtension{Raw: []byte(`{}`)},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "not found in spec.roleTemplates",
		},
		{
			name: "templateRef without templatePatch",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "base",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
					Roles: []RoleSpec{
						{
							Name:        "prefill",
							Replicas:    ptr.To(int32(1)),
							TemplateRef: &TemplateRef{Name: "base"},
							// Missing TemplatePatch
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "templatePatch: required when templateRef is set",
		},
		{
			// Priority mode: when templateRef is set, it takes precedence and template is ignored
			// This allows Go's zero-value PodTemplateSpec{} to coexist with templateRef
			name: "templateRef with non-empty template (priority mode - template ignored)",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					RoleTemplates: []RoleTemplate{
						{
							Name: "base",
							Template: corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
					Roles: []RoleSpec{
						{
							Name:        "prefill",
							Replicas:    ptr.To(int32(1)),
							TemplateRef: &TemplateRef{Name: "base"},
							Template: &corev1.PodTemplateSpec{ // Ignored when templateRef is set
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
							TemplatePatch: runtime.RawExtension{Raw: []byte(`{}`)},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "templatePatch without templateRef",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					Roles: []RoleSpec{
						{
							Name:     "prefill",
							Replicas: ptr.To(int32(1)),
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
							TemplatePatch: runtime.RawExtension{Raw: []byte(`{}`)}, // Should not be set
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "only valid when templateRef is set",
		},
		{
			name: "traditional mode (no templateRef, only template)",
			rbg: &RoleBasedGroup{
				Spec: RoleBasedGroupSpec{
					Roles: []RoleSpec{
						{
							Name:     "prefill",
							Replicas: ptr.To(int32(1)),
							Template: &corev1.PodTemplateSpec{
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{{Name: "app"}},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRoleTemplateReferences(tt.rbg)

			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRoleTemplateReferences() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("error message = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func TestIsDNSLabel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"valid lowercase", "nginx-base", true},
		{"valid with numbers", "sglang-v0-5-1", true},
		{"valid starting with number", "123-abc", true},
		{"invalid starts with hyphen", "-invalid", false},
		{"invalid ends with hyphen", "invalid-", false},
		{"invalid uppercase", "Has-Upper", false},
		{"invalid empty", "", false},
		{"invalid too long", strings.Repeat("a", 64), false},
		{"valid max length", strings.Repeat("a", 63), true},
		{"valid single char", "a", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isDNSLabel(tt.input); got != tt.want {
				t.Errorf("isDNSLabel(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
