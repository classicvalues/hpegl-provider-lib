package atf

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// GetAPIFunc accepts terraform states attributes as params and
// expects response and error as return values
type GetAPIFunc func(attr map[string]string) (interface{}, error)

type Acc struct {
	PreCheck     func(t *testing.T)
	Providers    map[string]*schema.Provider
	GetAPI       GetAPIFunc
	ResourceName string
	Version      string
}

// RunResourcePlanTest to run resource plan only test case. This will take first
// config from specific resource
func (a *Acc) RunResourcePlanTest(t *testing.T) {
	checkSkip(t)
	a.runPlanTest(t, true)
}

// RunDataSourceTests to run data source plan only test case. This will take first
// config from specific data source
func (a *Acc) RunDataSourceTests(t *testing.T) {
	r := newReader(t, false, a.ResourceName)
	checkSkip(t)
	testSteps := r.getTestCases(a.Version, a.GetAPI)

	resource.ParallelTest(t, resource.TestCase{
		IsUnitTest: false,
		PreCheck:   func() { a.PreCheck(t) },
		Providers:  a.Providers,
		Steps:      testSteps,
	})
}

// RunResourceTests creates test cases and run tests which includes create/update/delete/read
func (a *Acc) RunResourceTests(t *testing.T) {
	checkSkip(t)
	// populate test cases
	r := newReader(t, true, a.ResourceName)
	testSteps := r.getTestCases(a.Version, a.GetAPI)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { a.PreCheck(t) },
		Providers: a.Providers,
		Steps: testSteps,
	})
}

// runs plan test for resource or data source. only first config from test case
// will considered on plan test
func (a *Acc) runPlanTest(t *testing.T, isResource bool) {
	r := newReader(t, isResource, a.ResourceName)
	testSteps := r.getTestCases(a.Version, a.GetAPI)
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:  func() { a.PreCheck(t) },
		Providers: a.Providers,
		Steps: []resource.TestStep{
			{
				Config:             testSteps[0].Config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				Check:              testSteps[0].Check,
			},
		},
	})
}

func checkSkip(t *testing.T) {
	if strings.ToLower(os.Getenv("TF_ACC")) != "true" && os.Getenv("TF_ACC") != "1" {
		t.Skip("acceptance test is skipped since TF_ACC is not set")
	}
}
