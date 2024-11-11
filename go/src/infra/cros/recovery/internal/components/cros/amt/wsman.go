// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package amt implements just enough WS-Management to query and set the DUT's
// power state.
package amt

import (
	"fmt"

	"github.com/google/uuid"
)

// Base URL for CIM schema.
const schemaBase = "http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2"

// Sprintf format string used by createReadAMTPowerStateRequest.
const getPowerStateFmtString = `<?xml version="1.0" encoding="UTF-8"?>
   <s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:wsman="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd">
   <s:Header>
     <wsa:Action s:mustUnderstand="true">http://schemas.xmlsoap.org/ws/2004/09/transfer/Get</wsa:Action>
     <wsa:To s:mustUnderstand="true">%s</wsa:To>
     <wsman:ResourceURI s:mustUnderstand="true">%s/CIM_AssociatedPowerManagementService</wsman:ResourceURI>
     <wsa:MessageID s:mustUnderstand="true">uuid:%s</wsa:MessageID>
     <wsa:ReplyTo>
       <wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>
     </wsa:ReplyTo>
   </s:Header>
   <s:Body/>
</s:Envelope>`

// Sprintf format string used by createUpdateAMTPowerStateRequest.
const setPowerStateFmtString = `<?xml version="1.0" encoding="UTF-8"?>
    <s:Envelope xmlns:s="http://www.w3.org/2003/05/soap-envelope" xmlns:wsa="http://schemas.xmlsoap.org/ws/2004/08/addressing" xmlns:wsman="http://schemas.dmtf.org/wbem/wsman/1/wsman.xsd" xmlns:n1="http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_PowerManagementService">
    <s:Header>
    <wsa:Action s:mustUnderstand="true">http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_PowerManagementService/RequestPowerStateChange</wsa:Action>
    <wsa:To s:mustUnderstand="true">%s</wsa:To>
    <wsman:ResourceURI s:mustUnderstand="true">%s/CIM_PowerManagementService</wsman:ResourceURI>
    <wsa:MessageID s:mustUnderstand="true">uuid:%s</wsa:MessageID>
    <wsa:ReplyTo>
        <wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>
    </wsa:ReplyTo>
    <wsman:SelectorSet>
       <wsman:Selector Name="Name">Intel(r) AMT Power Management Service</wsman:Selector>
    </wsman:SelectorSet>
    </s:Header>
    <s:Body>
      <n1:RequestPowerStateChange_INPUT>
        <n1:PowerState>%d</n1:PowerState>
        <n1:ManagedElement>
          <wsa:Address>http://schemas.xmlsoap.org/ws/2004/08/addressing/role/anonymous</wsa:Address>
          <wsa:ReferenceParameters>
             <wsman:ResourceURI>http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_ComputerSystem</wsman:ResourceURI>
             <wsman:SelectorSet>
                <wsman:Selector wsman:Name="Name">ManagedSystem</wsman:Selector>
             </wsman:SelectorSet>
           </wsa:ReferenceParameters>
         </n1:ManagedElement>
       </n1:RequestPowerStateChange_INPUT>
      </s:Body></s:Envelope>`

// Returns the request body needed to get the power state via the CIM_AssociatedPowerManagementService.
func createReadAMTPowerStateRequest(uri string) string {
	return fmt.Sprintf(getPowerStateFmtString, uri, schemaBase, uuid.NewString())
}

// Returns the request body needed to set the power state via the CIM_PowerManagementService.
func createUpdateAMTPowerStateRequest(uri string, powerState int) string {
	return fmt.Sprintf(setPowerStateFmtString, uri, schemaBase, uuid.NewString(), powerState)
}
