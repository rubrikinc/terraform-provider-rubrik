// Copyright 2026 Rubrik, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to
// deal in the Software without restriction, including without limitation the
// rights to use, copy, modify, merge, publish, distribute, sublicense, and/or
// sell copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataCenterAWSAccountDataSource_validation(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: protoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
					data "rubrik_data_center_aws_account" "missing_name" {
					}
				`,
				ExpectError: regexp.MustCompile(`Missing required argument`),
			},
			{
				Config: `
					data "rubrik_data_center_aws_account" "empty_name" {
						name = ""
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Value`),
			},
			{
				Config: `
					data "rubrik_data_center_aws_account" "whitespace_name" {
						name = "   "
					}
				`,
				ExpectError: regexp.MustCompile(`Invalid Value`),
			},
			{
				Config: `
					data "polaris_data_center_aws_account" "missing_name" {
					}
				`,
				ExpectError: regexp.MustCompile(`Missing required argument`),
			},
		},
	})
}
