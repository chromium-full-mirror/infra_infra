// Copyright 2024 The Chromium Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package xmlrpc implements the XML-RPC client library.
package xmlrpc

import (
	"fmt"
	"reflect"

	"go.chromium.org/chromiumos/config/go/api/test/xmlrpc"
)

// PackArgsToXMLRPCValues packs values to XMLRPC structs.
func PackArgsToXMLRPCValues(values ...any) []*xmlrpc.Value {
	var r []*xmlrpc.Value
	for _, val := range values {
		if val == nil {
			continue
		}
		switch v := val.(type) {
		case string:
			r = append(r, &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_String_{
					String_: v,
				},
			})
		case bool:
			r = append(r, &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_Boolean{
					Boolean: v,
				},
			})
		case int:
			r = append(r, &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_Int{
					Int: int32(v),
				},
			})
		case float64:
			r = append(r, &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_Double{
					Double: v,
				},
			})
		default:
			// TODO(otabek@): Extend for more type if required. For now recovery is not using these types.
			message := fmt.Sprintf("%q is not a supported yet to be pack XMLRPC Value ", reflect.TypeOf(val))
			r = append(r, &xmlrpc.Value{
				ScalarOneof: &xmlrpc.Value_String_{
					String_: message,
				},
			})
		}
	}
	return r
}
