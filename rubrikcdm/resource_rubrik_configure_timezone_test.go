package rubrikcdm

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
)

func TestAccRubrikConfigureTimezone_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccRubrikConfigureTimezoneConfig("UTC"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("rubrik_configure_timezone.timezone", "timezone", "UTC"),
				),
			},
		},
	})
}

func testAccRubrikConfigureTimezoneConfig(timezone string) string {
	return fmt.Sprintf(`
resource "rubrik_configure_timezone" "timezone" {
	timezone = "%s"
	}
`, timezone)
}
