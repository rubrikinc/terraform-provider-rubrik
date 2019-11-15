package rubrikcdm

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccDataSourceRubrikClusterVersion_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			testAccCheckEnvVariables(t, []string{"rubrik_cdm_expected_version"})
		},
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccDataRubrikClusterVersion(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.rubrik_cluster_version.version", "cluster_version", os.Getenv("rubrik_cdm_expected_version")),
				),
			},
		},
	})
}

func testAccDataRubrikClusterVersion() string {
	return fmt.Sprintf(`
data "rubrik_cluster_version" "version" { }
`)
}
