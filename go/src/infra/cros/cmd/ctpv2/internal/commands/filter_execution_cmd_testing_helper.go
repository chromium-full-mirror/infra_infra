// Copyright 2023 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package commands

// NOTE: This is only to be used while testing ttcp filter with led.

// import (
// 	testapi "go.chromium.org/chromiumos/config/go/test/api"
// )

// addNTests returns N number of same test plan to test ttcp filter.
// func addNTests(n int) *testapi.InternalTestplan {
// 	testCases := []*testapi.CTPTestCase{}

// 	for i := 0; i < n; i++ {
// 		testCase := &testapi.CTPTestCase{
// 			//Name: fmt.Sprintf("test%d", i),
// 			Name: "tast.example.DDDTestOne",
// 			Metadata: &testapi.TestCaseMetadata{
// 				TestCase: &testapi.TestCase{
// 					Id: &testapi.TestCase_Id{
// 						// Value: fmt.Sprintf("tast.test%d", i),
// 						Value: "tast.example.DDDTestOne",
// 					},
// 					//Name: fmt.Sprintf("test%d", i),
// 					Name: "tast.example.DDDTestOne",
// 				},
// 				TestCaseExec: &testapi.TestCaseExec{
// 					TestHarness: &testapi.TestHarness{
// 						TestHarnessType: &testapi.TestHarness_Tast_{},
// 					},
// 				},
// 				TestCaseInfo: &testapi.TestCaseInfo{
// 					VariantCategory: &testapi.DDDVariantCategory{
// 						Value: "{\"name\": \"HWID:touchpad_field_vendor_id:distinct_values\"}",
// 					},
// 				},
// 			},
// 		}

// 		testCases = append(testCases, testCase)
// 	}

// 	return &testapi.InternalTestplan{
// 		TestCases: testCases,
// 		SuiteInfo: &testapi.SuiteInfo{
// 			SuiteMetadata: &testapi.SuiteMetadata{
// 				Channels: []string{
// 					"tot",
// 				},
// 				Pool: "DUT_POOL_QUOTA",
// 			},
// 			SuiteRequest: &testapi.SuiteRequest{
// 				SuiteRequest: &testapi.SuiteRequest_TestSuite{
// 					TestSuite: &testapi.TestSuite{
// 						Name: "xyz_suite",
// 						Spec: &testapi.TestSuite_TestCaseTagCriteria_{
// 							TestCaseTagCriteria: &testapi.TestSuite_TestCaseTagCriteria{
// 								Tags: []string{
// 									"group:mainline",
// 								},
// 								TagExcludes: []string{
// 									"informational",
// 								},
// 								TestNames: []string{
// 									"example",
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// }

// // addCustomTests returns custom test plan to test ttcp filter.
// func addCustomTests() *testapi.InternalTestplan {
// 	return &testapi.InternalTestplan{
// 		TestCases: []*testapi.CTPTestCase{
// 			{
// 				Name: "test0",
// 				Metadata: &testapi.TestCaseMetadata{
// 					TestCase: &testapi.TestCase{
// 						Id: &testapi.TestCase_Id{
// 							Value: "tast.test0",
// 						},
// 						Name: "test0",
// 					},
// 					TestCaseExec: &testapi.TestCaseExec{
// 						TestHarness: &testapi.TestHarness{
// 							TestHarnessType: &testapi.TestHarness_Tast_{},
// 						},
// 					},
// 					TestCaseInfo: &testapi.TestCaseInfo{
// 						VariantCategory: &testapi.DDDVariantCategory{
// 							Value: "{\"name\": \"WifiTeam:WiFi_MatFunc\"}",
// 						},
// 					},
// 				},
// 			},
// 			{
// 				Name: "test1",
// 				Metadata: &testapi.TestCaseMetadata{
// 					TestCase: &testapi.TestCase{
// 						Id: &testapi.TestCase_Id{
// 							Value: "tast.test1",
// 						},
// 						Name: "test1",
// 					},
// 					TestCaseExec: &testapi.TestCaseExec{
// 						TestHarness: &testapi.TestHarness{
// 							TestHarnessType: &testapi.TestHarness_Tast_{},
// 						},
// 					},
// 					TestCaseInfo: &testapi.TestCaseInfo{
// 						VariantCategory: &testapi.DDDVariantCategory{
// 							Value: "{\"name\": \"Common:context_agnostic\"}",
// 						},
// 					},
// 				},
// 			},
// 			{
// 				Name: "test2",
// 				Metadata: &testapi.TestCaseMetadata{
// 					TestCase: &testapi.TestCase{
// 						Id: &testapi.TestCase_Id{
// 							Value: "tast.test2",
// 						},
// 						Name: "test2",
// 					},
// 					TestCaseExec: &testapi.TestCaseExec{
// 						TestHarness: &testapi.TestHarness{
// 							TestHarnessType: &testapi.TestHarness_Tast_{},
// 						},
// 					},
// 					TestCaseInfo: &testapi.TestCaseInfo{
// 						VariantCategory: &testapi.DDDVariantCategory{
// 							Value: "{\"name\": \"HWID:touchpad_field_vendor_id:distinct_values\"}",
// 						},
// 					},
// 				},
// 			},
// 		},
// 		SuiteInfo: &testapi.SuiteInfo{
// 			SuiteMetadata: &testapi.SuiteMetadata{
// 				Channels: []string{
// 					"tot",
// 				},
// 				Pool: "DUT_POOL_QUOTA",
// 			},
// 			SuiteRequest: &testapi.SuiteRequest{
// 				SuiteRequest: &testapi.SuiteRequest_TestSuite{
// 					TestSuite: &testapi.TestSuite{
// 						Name: "xyz_suite",
// 						Spec: &testapi.TestSuite_TestCaseTagCriteria_{
// 							TestCaseTagCriteria: &testapi.TestSuite_TestCaseTagCriteria{
// 								Tags: []string{
// 									"group:mainline",
// 								},
// 								TagExcludes: []string{
// 									"informational",
// 								},
// 								TestNames: []string{
// 									"example",
// 								},
// 							},
// 						},
// 					},
// 				},
// 			},
// 		},
// 	}
// }
