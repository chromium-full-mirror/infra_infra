// Copyright (c) 2016 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package system

import (
	"testing"

	"github.com/shirou/gopsutil/v3/host"

	"go.chromium.org/luci/common/testing/ftt"
	"go.chromium.org/luci/common/testing/truth/assert"
	"go.chromium.org/luci/common/testing/truth/should"
)

func TestTemps(t *testing.T) {

	ftt.Run("ParseMacBookTemps", t, func(t *ftt.Test) {
		sensors := []host.TemperatureStat{
			{
				SensorKey:   "TA0P",
				Temperature: 11.1,
			},
			{
				SensorKey:   "TB0T",
				Temperature: 22.2,
			},
			{
				SensorKey:   "TC0P",
				Temperature: 33.3,
			},
		}
		temp := parseMacBookTemps(sensors)
		assert.Loosely(t, *temp.Ambient, should.Equal(11.1))
		assert.Loosely(t, *temp.Battery, should.Equal(22.2))
		assert.Loosely(t, len(temp.CPUs), should.Equal(1))
		assert.Loosely(t, temp.CPUs, should.Contain(cpuTemp{Core: "TC0P", Temperature: 33.3}))
	})

	ftt.Run("ParseMacBookMissingTemps", t, func(t *ftt.Test) {
		sensors := []host.TemperatureStat{
			{
				SensorKey:   "TA0P",
				Temperature: 0, // Ignored because temp is 0.
			},
			{
				SensorKey:   "ignored sensor",
				Temperature: 100.0,
			},
		}
		temp := parseMacBookTemps(sensors)
		assert.Loosely(t, temp.Ambient, should.BeNil)
		assert.Loosely(t, temp.Battery, should.BeNil)
		assert.Loosely(t, len(temp.CPUs), should.BeZero)
	})

	ftt.Run("ParsePowerEdgeTemps_windows", t, func(t *ftt.Test) {
		// Below string was pulled from the omreport of a R720 on Win7.
		out := []byte(`
		<?xml version="1.0" encoding="UTF-8"?>
                <OMA cli="true">
			<Chassis oid="2" status="4" name="2" objtype="17" index="0" display="Main System Chassis">
				<TemperatureProbeList poid="2" count="4">
					<TemperatureProbe oid="50331659" status="2" poid="2" pobjtype="17" index="0">
						<SubType>5</SubType>
						<ProbeReading>260</ProbeReading>
						<ProbeThresholds>
							<UNRThreshold>-2147483648</UNRThreshold>
							<UCThreshold>470</UCThreshold>
							<UNCThreshold>420</UNCThreshold>
							<LNCThreshold>30</LNCThreshold>
							<LCThreshold>-70</LCThreshold>
							<LNRThreshold>-2147483648</LNRThreshold>
						</ProbeThresholds>
						<ProbeStatus>2</ProbeStatus>
						<Capabilities>
							<ProbeUNCDefSetEnabled>true</ProbeUNCDefSetEnabled>
							<ProbeLNCDefSetEnabled>true</ProbeLNCDefSetEnabled>
							<ProbeUNCSetEnabled>true</ProbeUNCSetEnabled>
							<ProbeLNCSetEnabled>true</ProbeLNCSetEnabled>
						</Capabilities>
						<ProbeLocation>System Board Inlet Temp</ProbeLocation>
					</TemperatureProbe>
					<TemperatureProbe oid="50331660" status="2" poid="2" pobjtype="17" index="1">
						<SubType>5</SubType>
						<ProbeReading>450</ProbeReading>
						<ProbeThresholds>
							<UNRThreshold>-2147483648</UNRThreshold>
							<UCThreshold>750</UCThreshold>
							<UNCThreshold>700</UNCThreshold>
							<LNCThreshold>80</LNCThreshold>
							<LCThreshold>30</LCThreshold>
							<LNRThreshold>-2147483648</LNRThreshold>
						</ProbeThresholds>
						<ProbeStatus>2</ProbeStatus>
						<Capabilities>
							<ProbeUNCDefSetEnabled>false</ProbeUNCDefSetEnabled>
							<ProbeLNCDefSetEnabled>false</ProbeLNCDefSetEnabled>
							<ProbeUNCSetEnabled>false</ProbeUNCSetEnabled>
							<ProbeLNCSetEnabled>false</ProbeLNCSetEnabled>
						</Capabilities>
						<ProbeLocation>System Board Exhaust Temp</ProbeLocation>
					</TemperatureProbe>
					<TemperatureProbe oid="50331762" status="2" poid="2" pobjtype="17" index="2">
						<SubType>5</SubType>
						<ProbeReading>780</ProbeReading>
						<ProbeThresholds>
							<UNRThreshold>-2147483648</UNRThreshold>
							<UCThreshold>1000</UCThreshold>
							<UNCThreshold>950</UNCThreshold>
							<LNCThreshold>80</LNCThreshold>
							<LCThreshold>30</LCThreshold>
							<LNRThreshold>-2147483648</LNRThreshold>
						</ProbeThresholds>
						<ProbeStatus>2</ProbeStatus>
						<Capabilities>
							<ProbeUNCDefSetEnabled>false</ProbeUNCDefSetEnabled>
							<ProbeLNCDefSetEnabled>false</ProbeLNCDefSetEnabled>
							<ProbeUNCSetEnabled>false</ProbeUNCSetEnabled>
							<ProbeLNCSetEnabled>false</ProbeLNCSetEnabled>
						</Capabilities>
						<ProbeLocation>CPU1 Temp</ProbeLocation>
					</TemperatureProbe>
					<TemperatureProbe oid="50331763" status="2" poid="2" pobjtype="17" index="3">
						<SubType>5</SubType>
						<ProbeReading>800</ProbeReading>
						<ProbeThresholds>
							<UNRThreshold>-2147483648</UNRThreshold>
							<UCThreshold>1000</UCThreshold>
							<UNCThreshold>950</UNCThreshold>
							<LNCThreshold>80</LNCThreshold>
							<LCThreshold>30</LCThreshold>
							<LNRThreshold>-2147483648</LNRThreshold>
						</ProbeThresholds>
						<ProbeStatus>2</ProbeStatus>
						<Capabilities>
							<ProbeUNCDefSetEnabled>false</ProbeUNCDefSetEnabled>
							<ProbeLNCDefSetEnabled>false</ProbeLNCDefSetEnabled>
							<ProbeUNCSetEnabled>false</ProbeUNCSetEnabled>
							<ProbeLNCSetEnabled>false</ProbeLNCSetEnabled>
						</Capabilities>
						<ProbeLocation>CPU2 Temp</ProbeLocation>
					</TemperatureProbe>
				</TemperatureProbeList>
				<ObjStatus>2</ObjStatus>
			</Chassis>
			<SMStatus>0</SMStatus>
			<OMACMDNEW>0</OMACMDNEW>
                </OMA>
                `)
		temp, err := parsePowerEdgeTemps(out)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, *temp.Ambient, should.Equal(26.0))
		assert.Loosely(t, temp.Battery, should.BeNil)
		assert.Loosely(t, len(temp.CPUs), should.Equal(2))
		assert.Loosely(t, temp.CPUs, should.Contain(cpuTemp{Core: "CPU1 Temp", Temperature: 78.0}))
		assert.Loosely(t, temp.CPUs, should.Contain(cpuTemp{Core: "CPU2 Temp", Temperature: 80.0}))
	})

	ftt.Run("ParsePowerEdgeTemps_ubuntu", t, func(t *ftt.Test) {
		// Below string was pulled from the omreport of a R220 on Ubuntu Trusty.
		out := []byte(`
                <?xml version="1.0" encoding="UTF-8"?>
                <OMA cli="true">
                        <Chassis oid="2" status="2" name="2" objtype="17" index="0" display="Main System Chassis">
                                <TemperatureProbeList poid="2" count="1">
                                        <TemperatureProbe oid="134217730" status="2" poid="2" pobjtype="17" index="0">
                                                <SubType>5</SubType>
                                                <ProbeReading>275</ProbeReading>
                                                <ProbeThresholds>
                                                        <UNRThreshold>-2147483648</UNRThreshold>
                                                        <UCThreshold>470</UCThreshold>
                                                        <UNCThreshold>420</UNCThreshold>
                                                        <LNCThreshold>80</LNCThreshold>
                                                        <LCThreshold>30</LCThreshold>
                                                        <LNRThreshold>-2147483648</LNRThreshold>
                                                </ProbeThresholds>
                                                <ProbeStatus>2</ProbeStatus>
                                                <Capabilities>
                                                        <ProbeUNCDefSetEnabled>true</ProbeUNCDefSetEnabled>
                                                        <ProbeLNCDefSetEnabled>true</ProbeLNCDefSetEnabled>
                                                        <ProbeUNCSetEnabled>true</ProbeUNCSetEnabled>
                                                        <ProbeLNCSetEnabled>true</ProbeLNCSetEnabled>
                                                </Capabilities>
                                                <ProbeLocation>System Board Ambient Temp</ProbeLocation>
                                        </TemperatureProbe>
                                </TemperatureProbeList>
                                <ObjStatus>2</ObjStatus>
                        </Chassis>
                        <SMStatus>0</SMStatus>
                        <OMACMDNEW>0</OMACMDNEW>
                </OMA>
                `)
		temp, err := parsePowerEdgeTemps(out)
		assert.Loosely(t, err, should.BeNil)
		assert.Loosely(t, *temp.Ambient, should.Equal(27.5))
		assert.Loosely(t, temp.Battery, should.BeNil)
		assert.Loosely(t, len(temp.CPUs), should.BeZero)
	})
	ftt.Run("ParsePowerEdgeTempsBrokenXML", t, func(t *ftt.Test) {
		out := []byte("this isn't xml")
		temp, err := parsePowerEdgeTemps(out)
		assert.Loosely(t, err, should.NotBeNil)
		assert.Loosely(t, temp.Ambient, should.BeNil)
		assert.Loosely(t, temp.Battery, should.BeNil)
		assert.Loosely(t, len(temp.CPUs), should.BeZero)
	})

}
