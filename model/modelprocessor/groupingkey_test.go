// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package modelprocessor_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"hash"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/elastic/apm-data/model/modelpb"
	"github.com/elastic/apm-data/model/modelprocessor"
)

func TestSetGroupingKey(t *testing.T) {
	tests := map[string]struct {
		input       *modelpb.Error
		groupingKey string
	}{
		"empty": {
			input:       &modelpb.Error{},
			groupingKey: "",
		},
		"exception_type_log_parammessage": {
			input: &modelpb.Error{
				Exception: &modelpb.Exception{
					Type: "exception_type",
				},
				Log: &modelpb.ErrorLog{
					ParamMessage: "log_parammessage",
				},
			},
			groupingKey: hexifyStrings("exception_type", "log_parammessage"),
		},
		"exception_stacktrace": {
			input: &modelpb.Error{
				Exception: &modelpb.Exception{
					Stacktrace: []*modelpb.StacktraceFrame{
						{Module: "module", Filename: "filename", Classname: "classname", Function: "func_1"},
						{Filename: "filename", Classname: "classname", Function: "func_2"},
						{ExcludeFromGrouping: true, Function: "func_3"},
					},
					Cause: []*modelpb.Exception{{
						Stacktrace: []*modelpb.StacktraceFrame{
							{Classname: "classname", Function: "func_4"},
						},
						Cause: []*modelpb.Exception{{
							Stacktrace: []*modelpb.StacktraceFrame{
								{Function: "func_5"},
							},
						}},
					}, {
						Stacktrace: []*modelpb.StacktraceFrame{
							{Function: "func_6"},
						},
					}},
				},
				Log: &modelpb.ErrorLog{Stacktrace: []*modelpb.StacktraceFrame{{Filename: "abc"}}}, // ignored
			},
			groupingKey: hexifyStrings(
				"module", "func_1", "filename", "func_2", "classname", "func_4", "func_5", "func_6",
			),
		},
		"log_stacktrace": {
			input: &modelpb.Error{
				Log: &modelpb.ErrorLog{
					Stacktrace: []*modelpb.StacktraceFrame{{Function: "function"}},
				},
			},
			groupingKey: hexifyStrings("function"),
		},
		"exception_message": {
			input: &modelpb.Error{
				Exception: &modelpb.Exception{
					Message: "message_1",
					Cause: []*modelpb.Exception{{
						Message: "message_2",
						Cause: []*modelpb.Exception{
							{Message: "message_3"},
						},
					}, {
						Message: "message_4",
					}},
				},
				Log: &modelpb.ErrorLog{Message: "log_message"}, // ignored
			},
			groupingKey: hexifyStrings("message_1", "message_2", "message_3", "message_4"),
		},
		"log_message": {
			input: &modelpb.Error{
				Log: &modelpb.ErrorLog{Message: "log_message"}, // ignored
			},
			groupingKey: hexifyStrings("log_message"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			batch := modelpb.Batch{{
				Error: test.input,
			}}
			processor := modelprocessor.SetGroupingKey{NewHash: func() hash.Hash { return &fakeHash{} }}
			err := processor.ProcessBatch(context.Background(), &batch)
			assert.NoError(t, err)
			assert.Equal(t, test.groupingKey, batch[0].Error.GroupingKey)
		})
	}

}

// fakeHash is a hash.Hash that just accumulates the written data
// to a buffer, and returns it exactly as written.
type fakeHash struct {
	bytes.Buffer
}

func (f *fakeHash) Sum(b []byte) []byte {
	return append(b, f.Buffer.Bytes()...)
}

func (f *fakeHash) Size() int {
	return f.Buffer.Len()
}

func (f *fakeHash) BlockSize() int {
	return 1
}

func hexifyStrings(s ...string) string {
	return hex.EncodeToString([]byte(strings.Join(s, "")))
}
