// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package run

import (
	suschpb "go.chromium.org/chromiumos/infra/proto/go/testplans"

	"infra/cros/cmd/kron/common"
	"infra/cros/cmd/kron/configparser"
)

var (
	allowedConfigs = map[string]bool{
		"CrosboltPerfParallelsNightlyAutotest":       true,
		"DMServerEnrollmentDaily":                    true,
		"DMServerEnrollmentLiveFour":                 true,
		"DMServerEnrollmentLiveOne":                  true,
		"DMServerEnrollmentLiveTen":                  true,
		"DMServerZTEEnrollmentDaily":                 true,
		"DPanelEnd2EndPerBuild":                      true,
		"Daily_BTPerbuild_Update_Testbed":            true,
		"Daily_BT_Floss_Update_Testbed":              true,
		"Daily_BT_MTBF_Update_Testbed":               true,
		"DataLeakPreventionDMServerEnrollment":       true,
		"E2ECoverageTests":                           true,
		"EnableAutotestExperimentation":              true,
		"EnrollRetainment":                           true,
		"EnterpriseNightly":                          true,
		"FaftCr50DebugNightly":                       true,
		"FaftCr50Experimental":                       true,
		"FaftCr50TotNightly":                         true,
		"FaftDetachableNightly":                      true,
		"FaftGSCUnlaunchedNightly":                   true,
		"FaftGscNightly":                             true,
		"FlashromDaily":                              true,
		"FlexEnrollmentDaily":                        true,
		"GoldenTier0":                                true,
		"GoldenTier1":                                true,
		"GoldenTier2":                                true,
		"GoldenTier3":                                true,
		"GoldenTier4":                                true,
		"GoldenTier5":                                true,
		"GoldenVMCFT":                                true,
		"GraphicsDEQPPerDay":                         true,
		"GraphicsDEQPVK01PerDay":                     true,
		"GraphicsDEQPVK02PerDay":                     true,
		"GraphicsDEQPVK03PerDay":                     true,
		"GraphicsDEQPVK04PerDay":                     true,
		"GraphicsDEQPVK05PerDay":                     true,
		"GraphicsDEQPVK06PerDay":                     true,
		"GraphicsDEQPVK07PerDay":                     true,
		"GraphicsDEQPVK08PerDay":                     true,
		"GraphicsDEQPVK09PerDay":                     true,
		"GraphicsDEQPVK10PerDay":                     true,
		"GraphicsPerDay":                             true,
		"GraphicsPerDayKernelnext":                   true,
		"HMRDaily":                                   true,
		"HpsAtlDaily":                                true,
		"HpsSatlabDaily":                             true,
		"HpsSatlabSweetberryDaily":                   true,
		"HwsecNightly":                               true,
		"HwsecNightlyVM":                             true,
		"InputsOrcaDaily":                            true,
		"LanguagepacksHWRecognitionDlcDownloadDaily": true,
		"LauncherSearchQualityDaily":                 true,
		"MLBenchmarkNightly":                         true,
		"MfpPrintScan_17_Hour":                       true,
		"MfpPrintScan_23_Hour":                       true,
		"MfpPrintScan_2_Hour":                        true,
		"NetworkEndToEnd":                            true,
		"NetworkEndToEnd_Flaky":                      true,
		"NetworkPlatform":                            true,
		"NetworkPlatform_Flaky":                      true,
		"PowerDailyLoadTest_v11":                     true,
		"PowerDailyLoadTest_v11_LaCros":              true,
		"PowerDailyMisc":                             true,
		"PowerDailySegment":                          true,
		"PowerDailyVideoCall":                        true,
		"PowerDailyVideoEncode":                      true,
		"PowerTastDailyMisc":                         true,
		"PowerTastDailyVideoPlayback":                true,
		"Power_build":                                true,
		"PreprodDaily":                               true,
		"PreprodDailyVM":                             true,
		"ReportingTeamDaily":                         true,
		"SecuritySuite":                              true,
		"TastTestArgsExperimentation":                true,
		"TastTestVMArgsExperimentation":              true,
		"TastVmInfoNightly":                          true,
		"TestArgsExperimentation":                    true,
		"TopAppsDaily-0":                             true,
		"TopAppsDaily-1":                             true,
		"TopAppsDaily0":                              true,
		"TopAppsDaily3":                              true,
		"TopAppsDaily6":                              true,
		"VdiLimited":                                 true,
		"WeeklyPowerDaily":                           true,
		"WilcoBVE":                                   true,
		"WilcoBVEDock":                               true,
		"bluetooth_perbuild_beta__wificell_perbuild__bluetooth_wifi_coex__tauto__daily_hour_17":  true,
		"bluetooth_perbuild_beta__wificell_perbuild__bluetooth_wifi_coex__tfc__daily_hour_17":    true,
		"bluetooth_perbuild_canary__wificell_bt_perbuild__bluetooth_flaky__tauto__daily_hour_19": true,
		"bluetooth_perbuild_canary__wificell_bt_perbuild__bluetooth_flaky__tfc__daily_hour_19":   true,
		"bluetooth_perbuild_dev__wificell_perbuild__bluetooth_wifi_coex__tauto__daily_hour_17":   true,
		"bluetooth_perbuild_dev__wificell_perbuild__bluetooth_wifi_coex__tfc__daily_hour_17":     true,
		"bluetooth_standalone__MANAGED_POOL_QUOTA__bluetooth_sa2__tauto__daily_hour_15":          true,
		"bluetooth_standalone__MANAGED_POOL_QUOTA__bluetooth_sa__tauto__daily_hour_15":           true,
		"bluetooth_standalone__MANAGED_POOL_QUOTA__bluetooth_sa__tfc__daily_hour_15":             true,
		"bluetooth_wificell__wificell__bluetooth__tauto__daily_hour_18":                          true,
		"bluetooth_wificell__wificell__bluetooth__tfc__daily_hour_18":                            true,
		"bluetooth_wificell__wificell__bluetooth_floss__tauto__daily_hour_22":                    true,
		"bluetooth_wificell__wificell__bluetooth_floss__tfc__daily_hour_22":                      true,
		"bluetooth_wificell__wificell__bluetooth_wifi_coex__tauto__daily_hour_23":                true,
		"bluetooth_wificell__wificell__bluetooth_wifi_coex__tfc__daily_hour_23":                  true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth__tauto__daily_hour_18":               true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth__tfc__daily_hour_18":                 true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth_floss__tauto__daily_hour_20":         true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth_floss__tfc__daily_hour_20":           true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth_wifi_coex__tauto__daily_hour_23":     true,
		"bluetooth_wificell_kernelnext__wificell__bluetooth_wifi_coex__tfc__daily_hour_23":       true,
		"crosptsarm64": true,
		"crosptsx86":   true,
		"ddd__kernel_wifi_functional__wificell__wifi_matfunc_flaky__normal__tfc__daily_hour_10":      true,
		"ddd__kernel_wifi_functional__wificell__wifi_matfunc_flaky__persistence__tfc__daily_hour_10": true,
		"ddd__kernel_wifi_functional__wificell__wifi_matfunc_flaky__suspend__tfc__daily_hour_10":     true,
		"fingerprint-nightly": true,
		"kernel_wifi_chip_specific__Intel_JfP2__wificell__wifi_matfunc__normal__tfc__daily_hour_16":                        true,
		"kernel_wifi_chip_specific__Intel_JfP2__wificell__wifi_matfunc__persistence__tfc__daily_hour_16":                   true,
		"kernel_wifi_chip_specific__Intel_JfP2__wificell__wifi_matfunc__suspend__tfc__daily_hour_16":                       true,
		"kernel_wifi_chip_specific__Qualcomm_A6174A__wificell__wifi_matfunc__normal__tfc__daily_hour_16":                   true,
		"kernel_wifi_chip_specific__Qualcomm_A6174A__wificell__wifi_matfunc__persistence__tfc__daily_hour_16":              true,
		"kernel_wifi_chip_specific__Qualcomm_A6174A__wificell__wifi_matfunc__suspend__tfc__daily_hour_16":                  true,
		"kernel_wifi_chip_specific__Realtek_8822CE__wificell__wifi_matfunc__normal__tfc__daily_hour_16":                    true,
		"kernel_wifi_chip_specific__Realtek_8822CE__wificell__wifi_matfunc__persistence__tfc__daily_hour_16":               true,
		"kernel_wifi_chip_specific__Realtek_8822CE__wificell__wifi_matfunc__suspend__tfc__daily_hour_16":                   true,
		"kernel_wifi_connectivitynext__wificell_perbuild__wifi_matfunc__normal__tfc__daily_hour_16":                        true,
		"kernel_wifi_connectivitynext__wificell_perbuild__wifi_matfunc__persistence__tfc__daily_hour_16":                   true,
		"kernel_wifi_connectivitynext__wificell_perbuild__wifi_matfunc__suspend__tfc__daily_hour_16":                       true,
		"kernel_wifi_connectivitynext__wificell_perf__wifi_perf__tauto__daily_hour_16":                                     true,
		"kernel_wifi_functional__canary_daily__wificell__wifi_matfunc__normal__tfc__daily_hour_4":                          true,
		"kernel_wifi_functional__canary_daily__wificell__wifi_matfunc__persistence__tfc__daily_hour_4":                     true,
		"kernel_wifi_functional__canary_daily__wificell__wifi_matfunc__suspend__tfc__daily_hour_4":                         true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc__normal__tfc__daily_hour_4":                 true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc__persistence__tfc__daily_hour_4":            true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc__suspend__tfc__daily_hour_4":                true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc_flaky__normal__tfc__daily_hour_4":           true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc_flaky__persistence__tfc__daily_hour_4":      true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap__wifi_matfunc_flaky__suspend__tfc__daily_hour_4":          true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__normal__tfc__daily_hour_0":               true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__normal__tfc__daily_hour_17":              true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__persistence__tfc__daily_hour_0":          true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__persistence__tfc__daily_hour_17":         true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__suspend__tfc__daily_hour_0":              true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc__suspend__tfc__daily_hour_17":             true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__normal__tfc__daily_hour_0":         true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__normal__tfc__daily_hour_17":        true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__persistence__tfc__daily_hour_0":    true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__persistence__tfc__daily_hour_17":   true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__suspend__tfc__daily_hour_0":        true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_a__wifi_matfunc_flaky__suspend__tfc__daily_hour_17":       true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__normal__tfc__daily_hour_10":            true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__normal__tfc__daily_hour_12":            true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__normal__tfc__daily_hour_14":            true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__persistence__tfc__daily_hour_10":       true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__persistence__tfc__daily_hour_12":       true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__persistence__tfc__daily_hour_14":       true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__suspend__tfc__daily_hour_10":           true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__suspend__tfc__daily_hour_12":           true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc__suspend__tfc__daily_hour_14":           true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__normal__tfc__daily_hour_16":      true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__normal__tfc__daily_hour_18":      true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__normal__tfc__daily_hour_20":      true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__persistence__tfc__daily_hour_16": true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__persistence__tfc__daily_hour_18": true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__persistence__tfc__daily_hour_20": true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__suspend__tfc__daily_hour_16":     true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__suspend__tfc__daily_hour_18":     true,
		"kernel_wifi_functional__canary_daily__wificell_proto_ap_nuc__wifi_matfunc_flaky__suspend__tfc__daily_hour_20":     true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc__normal__tfc__daily_hour_4":                             true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc__persistence__tfc__daily_hour_4":                        true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc__suspend__tfc__daily_hour_4":                            true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc_flaky__normal__tfc__daily_hour_4":                       true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc_flaky__persistence__tfc__daily_hour_4":                  true,
		"kernel_wifi_functional_kernelnext__wificell__wifi_matfunc_flaky__suspend__tfc__daily_hour_4":                      true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc__normal__tfc__daily_hour_4":                    true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc__persistence__tfc__daily_hour_4":               true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc__suspend__tfc__daily_hour_4":                   true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc_flaky__normal__tfc__daily_hour_4":              true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc_flaky__persistence__tfc__daily_hour_4":         true,
		"kernel_wifi_functional_kernelnext__wificell_proto_ap__wifi_matfunc_flaky__suspend__tfc__daily_hour_4":             true,
		"kernel_wifi_groamer__groamer__wifi_atten_perf__tauto__daily_hour_4":                                               true,
		"kernel_wifi_groamer__groamer__wifi_atten_roam_perf__tfc__daily_hour_18":                                           true,
		"kernel_wifi_groamer__groamer_proto__wifi_atten_perf__tauto__daily_hour_4":                                         true,
		"kernel_wifi_performance__wificell_perf__wifi_perf__tauto__daily_hour_1":                                           true,
		"kernel_wifi_performance__wificell_perf__wifi_perf__tfc__daily_hour_1":                                             true,
		"kernel_wifi_performance__wificell_perf__wifi_perf_flaky__tauto__daily_hour_5":                                     true,
		"kernel_wifi_performance__wificell_perf__wifi_perf_flaky__tfc__daily_hour_5":                                       true,
		"kernel_wifi_performance__wificell_proto_ap__wifi_perf__tfc__daily_hour_1":                                         true,
		"kernel_wifi_performance__wificell_proto_ap__wifi_perf_flaky__tfc__daily_hour_5":                                   true,
		"kernel_wifi_performance__wificell_proto_ap__wifi_perf_openwrt__tauto__daily_hour_1":                               true,
		"kernel_wifi_performance__wificell_proto_ap__wifi_perf_openwrt_flaky__tauto__daily_hour_5":                         true,
		"kernel_wifi_performance__wificell_proto_ap_nuc__wifi_perf__tfc__daily_hour_1":                                     true,
		"kernel_wifi_performance__wificell_proto_ap_nuc__wifi_perf_flaky__tfc__daily_hour_1":                               true,
		"kernel_wifi_performance__wificell_proto_ap_nuc__wifi_perf_flaky__tfc__daily_hour_5":                               true,
		"kernel_wifi_performance__wificell_proto_ap_nuc__wifi_perf_flaky__tfc__daily_hour_9":                               true,
		"kernel_wifi_performance_kernelnext__wificell_perf__wifi_perf__tauto__daily_hour_1":                                true,
		"kernel_wifi_performance_kernelnext__wificell_perf__wifi_perf__tfc__daily_hour_1":                                  true,
		"kernel_wifi_stress_test__stress-wifi__wifi_stress__tauto__daily_hour_17":                                          true,
		"schedukeDev": true,
		"video":       true,
		"wifi_commercial__canary_daily__wificell__wifi_commercial__tfc__daily_hour_6":       true,
		"wifi_commercial__canary_daily__wificell__wifi_commercial_flaky__tfc__daily_hour_6": true,
		"wifi_endtoend_daily__wificell__wifi_endtoend__tfc__daily_hour_15":                  true,
		"wifi_endtoend_daily__wificell__wifi_endtoend_flaky__tfc__daily_hour_15":            true,
	}
)

// isAllowed checks the migration rules to determine if a config has been
// migrated to Kron or not.
func isAllowed(config *suschpb.SchedulerConfig) bool {
	// Disallow partner configs.
	if config.GetRunOptions().GetBuilderId().GetProject() != "" && config.GetRunOptions().GetBuilderId().GetBucket() != "" && config.GetRunOptions().GetBuilderId().GetBuilder() != "" {
		return false
	}

	// Disallow multi-dut and firmware configs.
	if configparser.IsMultiDut(config) || configparser.IsFirmware(config) {
		return false
	}

	// Allow NEW_BUILD configs.
	if config.GetLaunchCriteria().GetLaunchProfile() == suschpb.SchedulerConfig_LaunchCriteria_NEW_BUILD {
		return true
	}

	// Allow Explicitly included configs.
	if _, ok := allowedConfigs[config.GetName()]; ok {
		return true
	}

	// Exclude everything else.
	return false
}

// filterConfigs iterates through the triggered SuSch Configs and scrubs out all
// configs which are not on the allowlist.
//
// TODO(b/319273876): Remove slow migration logic upon completion of
// transition from SuiteScheduler to Kron.
func filterConfigs(configs []*suschpb.SchedulerConfig) []*suschpb.SchedulerConfig {
	filteredMap := []*suschpb.SchedulerConfig{}

	// Check each triggered config to ensure that they are not disallowed by the
	// current migration rules.
	for _, config := range configs {
		if !isAllowed(config) {
			common.Stdout.Printf("Config %s was filtered out by the current migration rules.", config.Name)
			continue
		}

		filteredMap = append(filteredMap, config)
	}

	return filteredMap
}
