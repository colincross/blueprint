// Copyright 2014 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proptools

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/google/blueprint/parser"
)

var validUnpackTestCases = []struct {
	name   string
	input  string
	output []interface{}
	empty  []interface{}
	errs   []error
}{
	{
		name: "blank and unset",
		input: `
			m {
				s: "abc",
				blank: "",
			}
		`,
		output: []interface{}{
			&struct {
				S     *string
				Blank *string
				Unset *string
			}{
				S:     StringPtr("abc"),
				Blank: StringPtr(""),
				Unset: nil,
			},
		},
	},

	{
		name: "string",
		input: `
			m {
				s: "abc",
			}
		`,
		output: []interface{}{
			&struct {
				S string
			}{
				S: "abc",
			},
		},
	},

	{
		name: "bool",
		input: `
			m {
				isGood: true,
			}
		`,
		output: []interface{}{
			&struct {
				IsGood bool
			}{
				IsGood: true,
			},
		},
	},

	{
		name: "boolptr",
		input: `
			m {
				isGood: true,
				isBad: false,
			}
		`,
		output: []interface{}{
			&struct {
				IsGood *bool
				IsBad  *bool
				IsUgly *bool
			}{
				IsGood: BoolPtr(true),
				IsBad:  BoolPtr(false),
				IsUgly: nil,
			},
		},
	},

	{
		name: "slice",
		input: `
			m {
				stuff: ["asdf", "jkl;", "qwert",
					"uiop", "bnm,"],
				empty: []
			}
		`,
		output: []interface{}{
			&struct {
				Stuff     []string
				Empty     []string
				Nil       []string
				NonString []struct{ S string } `blueprint:"mutated"`
			}{
				Stuff:     []string{"asdf", "jkl;", "qwert", "uiop", "bnm,"},
				Empty:     []string{},
				Nil:       nil,
				NonString: nil,
			},
		},
	},

	{
		name: "double nested",
		input: `
			m {
				nested: {
					nested: {
						s: "abc",
					},
				},
			}
		`,
		output: []interface{}{
			&struct {
				Nested struct {
					Nested struct {
						S string
					}
				}
			}{
				Nested: struct{ Nested struct{ S string } }{
					Nested: struct{ S string }{
						S: "abc",
					},
				},
			},
		},
	},

	{
		name: "nested",
		input: `
			m {
				nested: {
					s: "abc",
				}
			}
		`,
		output: []interface{}{
			&struct {
				Nested struct {
					S string
				}
			}{
				Nested: struct{ S string }{
					S: "abc",
				},
			},
		},
	},

	{
		name: "nested interface",
		input: `
			m {
				nested: {
					s: "def",
				}
			}
		`,
		output: []interface{}{
			&struct {
				Nested interface{}
			}{
				Nested: &struct{ S string }{
					S: "def",
				},
			},
		},
	},

	{
		name: "mixed",
		input: `
			m {
				nested: {
					foo: "abc",
				},
				bar: false,
				baz: ["def", "ghi"],
			}
		`,
		output: []interface{}{
			&struct {
				Nested struct {
					Foo string
				}
				Bar bool
				Baz []string
			}{
				Nested: struct{ Foo string }{
					Foo: "abc",
				},
				Bar: false,
				Baz: []string{"def", "ghi"},
			},
		},
	},

	{
		name: "filter",
		input: `
			m {
				nested: {
					foo: "abc",
				},
				bar: false,
				baz: ["def", "ghi"],
			}
		`,
		output: []interface{}{
			&struct {
				Nested struct {
					Foo string `allowNested:"true"`
				} `blueprint:"filter(allowNested:\"true\")"`
				Bar bool
				Baz []string
			}{
				Nested: struct {
					Foo string `allowNested:"true"`
				}{
					Foo: "abc",
				},
				Bar: false,
				Baz: []string{"def", "ghi"},
			},
		},
	},

	// Anonymous struct
	{
		name: "embedded struct",
		input: `
			m {
				s: "abc",
				nested: {
					s: "def",
				},
			}
		`,
		output: []interface{}{
			&struct {
				EmbeddedStruct
				Nested struct {
					EmbeddedStruct
				}
			}{
				EmbeddedStruct: EmbeddedStruct{
					S: "abc",
				},
				Nested: struct {
					EmbeddedStruct
				}{
					EmbeddedStruct: EmbeddedStruct{
						S: "def",
					},
				},
			},
		},
	},

	// Anonymous interface
	{
		name: "embedded interface",
		input: `
			m {
				s: "abc",
				nested: {
					s: "def",
				},
			}
		`,
		output: []interface{}{
			&struct {
				EmbeddedInterface
				Nested struct {
					EmbeddedInterface
				}
			}{
				EmbeddedInterface: &struct{ S string }{
					S: "abc",
				},
				Nested: struct {
					EmbeddedInterface
				}{
					EmbeddedInterface: &struct{ S string }{
						S: "def",
					},
				},
			},
		},
	},

	// Anonymous struct with name collision
	{
		name: "embedded name collision",
		input: `
			m {
				s: "abc",
				nested: {
					s: "def",
				},
			}
		`,
		output: []interface{}{
			&struct {
				S string
				EmbeddedStruct
				Nested struct {
					S string
					EmbeddedStruct
				}
			}{
				S: "abc",
				EmbeddedStruct: EmbeddedStruct{
					S: "abc",
				},
				Nested: struct {
					S string
					EmbeddedStruct
				}{
					S: "def",
					EmbeddedStruct: EmbeddedStruct{
						S: "def",
					},
				},
			},
		},
	},

	// Anonymous interface with name collision
	{
		name: "embeded interface name collision",
		input: `
			m {
				s: "abc",
				nested: {
					s: "def",
				},
			}
		`,
		output: []interface{}{
			&struct {
				S string
				EmbeddedInterface
				Nested struct {
					S string
					EmbeddedInterface
				}
			}{
				S: "abc",
				EmbeddedInterface: &struct{ S string }{
					S: "abc",
				},
				Nested: struct {
					S string
					EmbeddedInterface
				}{
					S: "def",
					EmbeddedInterface: &struct{ S string }{
						S: "def",
					},
				},
			},
		},
	},

	// Variables
	{
		name: "variables",
		input: `
			list = ["abc"]
			string = "def"
			list_with_variable = [string]
			m {
				s: string,
				list: list,
				list2: list_with_variable,
			}
		`,
		output: []interface{}{
			&struct {
				S     string
				List  []string
				List2 []string
			}{
				S:     "def",
				List:  []string{"abc"},
				List2: []string{"def"},
			},
		},
	},

	// Multiple property structs
	{
		name: "multiple",
		input: `
			m {
				nested: {
					s: "abc",
				}
			}
		`,
		output: []interface{}{
			&struct {
				Nested struct {
					S string
				}
			}{
				Nested: struct{ S string }{
					S: "abc",
				},
			},
			&struct {
				Nested struct {
					S string
				}
			}{
				Nested: struct{ S string }{
					S: "abc",
				},
			},
			&struct {
			}{},
		},
	},

	// Nil pointer to struct
	{
		name: "nil struct pointer",
		input: `
			m {
				nested: {
					s: "abc",
				}
			}
		`,
		output: []interface{}{
			&struct {
				Nested *struct {
					S string
				}
			}{
				Nested: &struct{ S string }{
					S: "abc",
				},
			},
		},
		empty: []interface{}{
			&struct {
				Nested *struct {
					S string
				}
			}{},
		},
	},

	// Interface containing nil pointer to struct
	{
		name: "interface nil struct pointer",
		input: `
			m {
				nested: {
					s: "abc",
				}
			}
		`,
		output: []interface{}{
			&struct {
				Nested interface{}
			}{
				Nested: &EmbeddedStruct{
					S: "abc",
				},
			},
		},
		empty: []interface{}{
			&struct {
				Nested interface{}
			}{
				Nested: (*EmbeddedStruct)(nil),
			},
		},
	},

	// Factory set properties
	{
		name: "factory properties",
		input: `
			m {
				string: "abc",
				string_ptr: "abc",
				bool: false,
				bool_ptr: false,
				list: ["a", "b", "c"],
			}
		`,
		output: []interface{}{
			&struct {
				String     string
				String_ptr *string
				Bool       bool
				Bool_ptr   *bool
				List       []string
			}{
				String:     "012abc",
				String_ptr: StringPtr("abc"),
				Bool:       true,
				Bool_ptr:   BoolPtr(false),
				List:       []string{"0", "1", "2", "a", "b", "c"},
			},
		},
		empty: []interface{}{
			&struct {
				String     string
				String_ptr *string
				Bool       bool
				Bool_ptr   *bool
				List       []string
			}{
				String:     "012",
				String_ptr: StringPtr("012"),
				Bool:       true,
				Bool_ptr:   BoolPtr(true),
				List:       []string{"0", "1", "2"},
			},
		},
	},
	// Captitalized property
	{
		input: `
			m {
				CAPITALIZED: "foo",
			}
		`,
		output: []interface{}{
			&struct {
				CAPITALIZED string
			}{
				CAPITALIZED: "foo",
			},
		},
	},
}

func TestUnpackProperties(t *testing.T) {
	for _, testCase := range validUnpackTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := bytes.NewBufferString(testCase.input)
			file, errs := parser.ParseAndEval("", r, parser.NewScope(nil))
			if len(errs) != 0 {
				t.Errorf("test case: %s", testCase.input)
				t.Errorf("unexpected parse errors:")
				for _, err := range errs {
					t.Errorf("  %s", err)
				}
				t.FailNow()
			}

			for _, def := range file.Defs {
				module, ok := def.(*parser.Module)
				if !ok {
					continue
				}

				var output []interface{}
				if len(testCase.empty) > 0 {
					for _, p := range testCase.empty {
						output = append(output, CloneProperties(reflect.ValueOf(p)).Interface())
					}
				} else {
					for _, p := range testCase.output {
						output = append(output, CloneEmptyProperties(reflect.ValueOf(p)).Interface())
					}
				}

				_, errs = UnpackProperties(module.Properties, output...)
				if len(errs) != 0 && len(testCase.errs) == 0 {
					t.Errorf("test case: %s", testCase.input)
					t.Errorf("unexpected unpack errors:")
					for _, err := range errs {
						t.Errorf("  %s", err)
					}
					t.FailNow()
				} else if !reflect.DeepEqual(errs, testCase.errs) {
					t.Errorf("test case: %s", testCase.input)
					t.Errorf("incorrect errors:")
					t.Errorf("  expected: %+v", testCase.errs)
					t.Errorf("       got: %+v", errs)
				}

				if len(output) != len(testCase.output) {
					t.Fatalf("incorrect number of property structs, expected %d got %d",
						len(testCase.output), len(output))
				}

				for i := range output {
					got := reflect.ValueOf(output[i]).Interface()
					if !reflect.DeepEqual(got, testCase.output[i]) {
						t.Errorf("test case: %s", testCase.input)
						t.Errorf("incorrect output:")
						t.Errorf("  expected: %+v", testCase.output[i])
						t.Errorf("       got: %+v", got)
					}
				}
			}
		})
	}
}

func TestUnpackErrors(t *testing.T) {
	testCases := []struct {
		name   string
		input  string
		output []interface{}
		errors []string
	}{
		{
			name: "missing",
			input: `
				m {
					missing: true,
				}
			`,
			output: []interface{}{},
			errors: []string{`<input>:3:13: unrecognized property "missing"`},
		},
		{
			name: "missing nested",
			input: `
				m {
					nested: {
						missing: true,
					},
				}
			`,
			output: []interface{}{
				&struct {
					Nested struct{}
				}{},
			},
			errors: []string{`<input>:4:14: unrecognized property "nested.missing"`},
		},
		{
			name: "mutated",
			input: `
				m {
					mutated: true,
				}
			`,
			output: []interface{}{
				&struct {
					Mutated bool `blueprint:"mutated"`
				}{},
			},
			errors: []string{`<input>:3:13: mutated field mutated cannot be set in a Blueprint file`},
		},
		{
			name: "nested mutated",
			input: `
				m {
					nested: {
						mutated: true,
					},
				}
			`,
			output: []interface{}{
				&struct {
					Nested struct {
						Mutated bool `blueprint:"mutated"`
					}
				}{},
			},
			errors: []string{`<input>:4:14: mutated field nested.mutated cannot be set in a Blueprint file`},
		},
		{
			name: "duplicate",
			input: `
				m {
					exists: true,
					exists: true,
				}
			`,
			output: []interface{}{
				&struct {
					Exists bool
				}{},
			},
			errors: []string{
				`<input>:4:12: property "exists" already defined`,
				`<input>:3:12: <-- previous definition here`,
			},
		},
		{
			name: "nested duplicate",
			input: `
				m {
					nested: {
						exists: true,
						exists: true,
					},
				}
			`,
			output: []interface{}{
				&struct {
					Nested struct {
						Exists bool
					}
				}{},
			},
			errors: []string{
				`<input>:5:13: property "nested.exists" already defined`,
				`<input>:4:13: <-- previous definition here`,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			r := bytes.NewBufferString(testCase.input)
			file, errs := parser.ParseAndEval("", r, parser.NewScope(nil))
			if len(errs) != 0 {
				t.Errorf("test case: %s", testCase.input)
				t.Errorf("unexpected parse errors:")
				for _, err := range errs {
					t.Errorf("  %s", err)
				}
				t.FailNow()
			}

			for _, def := range file.Defs {
				module, ok := def.(*parser.Module)
				if !ok {
					continue
				}

				var output []interface{}
				for _, p := range testCase.output {
					output = append(output, CloneEmptyProperties(reflect.ValueOf(p)).Interface())
				}

				_, errs = UnpackProperties(module.Properties, output...)

				printErrors := false
				for _, expectedErr := range testCase.errors {
					foundError := false
					for _, err := range errs {
						if err.Error() == expectedErr {
							foundError = true
						}
					}
					if !foundError {
						t.Errorf("expected error %s", expectedErr)
						printErrors = true
					}
				}
				if printErrors {
					t.Errorf("got errors:")
					for _, err := range errs {
						t.Errorf("   %s", err.Error())
					}
				}
			}
		})
	}
}

func BenchmarkUnpackProperties(b *testing.B) {
	run := func(b *testing.B, props []interface{}, input string) {
		b.ReportAllocs()
		b.StopTimer()
		r := bytes.NewBufferString(input)
		file, errs := parser.ParseAndEval("", r, parser.NewScope(nil))
		if len(errs) != 0 {
			b.Errorf("test case: %s", input)
			b.Errorf("unexpected parse errors:")
			for _, err := range errs {
				b.Errorf("  %s", err)
			}
			b.FailNow()
		}

		for i := 0; i < b.N; i++ {
			for _, def := range file.Defs {
				module, ok := def.(*parser.Module)
				if !ok {
					continue
				}

				var output []interface{}
				for _, p := range props {
					output = append(output, CloneProperties(reflect.ValueOf(p)).Interface())
				}

				b.StartTimer()
				_, errs = UnpackProperties(module.Properties, output...)
				b.StopTimer()
				if len(errs) > 0 {
					b.Errorf("unexpected unpack errors:")
					for _, err := range errs {
						b.Errorf("  %s", err)
					}
				}
			}
		}
	}

	b.Run("basic", func(b *testing.B) {
		props := []interface{}{
			&struct {
				Nested struct {
					S string
				}
			}{},
		}
		bp := `
			m {
				nested: {
					s: "abc",
				},
			}
		`
		run(b, props, bp)
	})

	b.Run("interface", func(b *testing.B) {
		props := []interface{}{
			&struct {
				Nested interface{}
			}{
				Nested: (*struct {
					S string
				})(nil),
			},
		}
		bp := `
			m {
				nested: {
					s: "abc",
				},
			}
		`
		run(b, props, bp)
	})

	b.Run("many", func(b *testing.B) {
		props := []interface{}{
			&struct {
				A *string
				B *string
				C *string
				D *string
				E *string
				F *string
				G *string
				H *string
				I *string
				J *string
			}{},
		}
		bp := `
			m {
				a: "a",
				b: "b",
				c: "c",
				d: "d",
				e: "e",
				f: "f",
				g: "g",
				h: "h",
				i: "i",
				j: "j",
			}
		`
		run(b, props, bp)
	})

	b.Run("deep", func(b *testing.B) {
		props := []interface{}{
			&struct {
				Nested struct {
					Nested struct {
						Nested struct {
							Nested struct {
								Nested struct {
									Nested struct {
										Nested struct {
											Nested struct {
												Nested struct {
													Nested struct {
														S string
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}{},
		}
		bp := `
			m {
				nested: { nested: { nested: { nested: { nested: {
					nested: { nested: { nested: { nested: { nested: {
						s: "abc",
					}, }, }, }, },
				}, }, }, }, },
			}
		`
		run(b, props, bp)
	})
}
