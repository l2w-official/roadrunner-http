package handler

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

var samples = []struct { //nolint:gochecknoglobals
	in  string
	out []string
}{
	{"key", []string{"key"}},
	{"key[subkey]", []string{"key", "subkey"}},
	{"key[subkey]value", []string{"key", "subkey", "value"}},
	{"key[subkey][value]", []string{"key", "subkey", "value"}},
	{"key[subkey][value][]", []string{"key", "subkey", "value", ""}},
	{"key[subkey] [value][]", []string{"key", "subkey", "value", ""}},
	{"key [ subkey ] [ value ] [ ]", []string{"key", "subkey", "value", ""}},
	{"ключь [ subkey ] [ value ] [ ]", []string{"ключь", "subkey", "value", ""}}, // test non 1-byte symbols
	{"options[0][name]", []string{"options", "0", "name"}},
}

func Test_FetchIndexes(t *testing.T) {
	for i := range samples {
		keys := make([]string, 1)
		fetchIndexes(samples[i].in, &keys)
		if !same(keys, samples[i].out) {
			t.Errorf("got %q, want %q", keys, samples[i].out)
		}
	}
}

func BenchmarkConfig_FetchIndexes(b *testing.B) {
	b.ReportAllocs()
	for _, tt := range samples {
		for b.Loop() {
			keys := make([]string, 1)
			fetchIndexes(tt.in, &keys)
			if !same(keys, tt.out) {
				b.Fail()
			}
		}
	}
}

func same(in, out []string) bool {
	if len(in) != len(out) {
		return false
	}

	for i := range in {
		if in[i] != out[i] {
			return false
		}
	}

	return true
}

func TestDataTreePush(t *testing.T) {
	type orderedData []struct {
		key   string
		value []string
	}
	testCases := []struct {
		name    string
		values  orderedData
		wantVal any
		wantErr error
	}{
		{
			name: "the longest chain is visible in tree structure",
			values: orderedData{
				{
					key:   "key[questions][2]",
					value: []string{""},
				},
				{
					key:   "key[questions][2][answers][3][clue]",
					value: []string{""},
				},
			},
			wantVal: dataTree{
				"questions": dataTree{
					"2": dataTree{
						"answers": dataTree{
							"3": dataTree{
								"clue": "",
							},
						},
					},
				},
			},
		},
		{
			name: "values from chain which contains shorter chains will have high priority",
			values: orderedData{

				{
					key:   "key[questions][10][answers][3][clue]",
					value: []string{"wwwww"},
				},
				{
					key:   "key[questions][10]",
					value: []string{""},
				},
				{
					key:   "key[questions][10][answers][4][clue]",
					value: []string{"12345"},
				},
			},
			wantVal: dataTree{
				"questions": dataTree{
					"10": dataTree{
						"answers": dataTree{
							"3": dataTree{
								"clue": "wwwww",
							},
							"4": dataTree{
								"clue": "12345",
							},
						},
					},
				},
			},
		},
		{
			name: "chain with same prefix and empty value should be overwriteen by full path",
			values: orderedData{
				{
					key:   "key[questions][5]",
					value: []string{""},
				},
				{
					key:   "key[questions][5][answers][3]",
					value: []string{""},
				},
				{
					key:   "key[questions][5][answers][3][clue]",
					value: []string{"xxxxx"},
				},
			},
			wantVal: dataTree{
				"questions": dataTree{
					"5": dataTree{
						"answers": dataTree{
							"3": dataTree{
								"clue": "xxxxx",
							},
						},
					},
				},
			},
		},
		{
			name: "chains with similar structure should fail when all has assigned value",
			values: orderedData{
				{
					key:   "key[questions][5]",
					value: []string{"1"},
				},
				{
					key:   "key[questions][5][answers][3][clue]",
					value: []string{"2"},
				},
			},
			wantErr: errors.New("invalid multiple values to key '5' in tree"),
		},
		{
			name: "non associated array should stay",
			values: orderedData{
				{
					key:   "key[]",
					value: []string{"value2"},
				},
				{
					key:   "key",
					value: []string{""},
				},
			},
			wantVal: []string{"value2"},
		},
		{
			name: "old value should get overwritten by not empty value",
			values: orderedData{
				{
					key:   "key[]",
					value: []string{"value2"},
				},
				{
					key:   "key",
					value: []string{"value1"},
				},
			},
			wantVal: "value1",
		},
		{
			name: "empty string should get overwritten by new dataTree",
			values: orderedData{
				{
					key:   "key",
					value: []string{""},
				},
				{
					key:   "key[options][id]",
					value: []string{"id1"},
				},
				{
					key:   "key[options][value]",
					value: []string{"value1"},
				},
			},
			wantVal: dataTree{
				"options": dataTree{
					"id":    "id1",
					"value": "value1",
				},
			},
		},
		{
			name: "dataTree should not get overwritten by empty string",
			values: orderedData{
				{
					key:   "key[options][id]",
					value: []string{"id1"},
				},
				{
					key:   "key[options][value]",
					value: []string{"value1"},
				},
				{
					key:   "key[]",
					value: []string{""},
				},
			},
			wantVal: dataTree{
				"options": dataTree{
					"id":    "id1",
					"value": "value1",
				},
			},
		},
		{
			name: "there should be error if dataTree goes before scalar value",
			values: orderedData{
				{
					key:   "key[options][id]",
					value: []string{"id1"},
				},
				{
					key:   "key[options][value]",
					value: []string{"value1"},
				},
				{
					key:   "key",
					value: []string{"value"},
				},
			},
			wantErr: errors.New("invalid multiple values to key 'key' in tree"),
		},
		{
			name: "there should be error if scalar value goes before dataTree",
			values: orderedData{
				{
					key:   "key",
					value: []string{"value"},
				},
				{
					key:   "key[options][id]",
					value: []string{"id1"},
				},
				{
					key:   "key[options][value]",
					value: []string{"value1"},
				},
			},
			wantErr: errors.New("invalid multiple values to key 'key' in tree"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var (
				d   = make(dataTree)
				err error
			)

			for _, v := range tt.values {
				err = d.push(v.key, v.value)
				if err != nil {
					break
				}
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %+v but got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Fatalf("want err %+v but got err %+v", tt.wantErr, err)
				}

				return
			}
			if err != nil {
				t.Fatalf("want no err but got err %+v", err)
			}
			if diff := cmp.Diff(d["key"], tt.wantVal); len(diff) > 0 {
				t.Fatalf("diff should be empty: %+v", diff)
			}
		})
	}
}

func TestFileTreePush(t *testing.T) {
	type orderedData []struct {
		key   string
		value []*FileUpload
	}
	testCases := []struct {
		name    string
		values  orderedData
		wantVal any
		wantErr error
	}{
		{
			name: "non associated array should stay",
			values: orderedData{
				{
					key: "key[]",
					value: []*FileUpload{
						{
							Name: "value2",
						},
					},
				},
				{
					key:   "key",
					value: []*FileUpload{},
				},
			},
			wantVal: []*FileUpload{
				{
					Name: "value2",
				},
			},
		},
		{
			name: "old value should get overwritten by not empty value",
			values: orderedData{
				{
					key: "key[]",
					value: []*FileUpload{
						{
							Name: "value2",
						},
					},
				},
				{
					key: "key",
					value: []*FileUpload{
						{
							Name: "value1",
						},
					},
				},
			},
			wantVal: &FileUpload{Name: "value1"},
		},
		{
			name: "empty value should get overwritten by new fileTree",
			values: orderedData{
				{
					key:   "key",
					value: []*FileUpload{},
				},
				{
					key: "key[options][id]",
					value: []*FileUpload{
						{
							Name: "id1",
						},
					},
				},
				{
					key: "key[options][value]",
					value: []*FileUpload{
						{
							Name: "value1",
						},
					},
				},
			},
			wantVal: fileTree{
				"options": fileTree{
					"id":    &FileUpload{Name: "id1"},
					"value": &FileUpload{Name: "value1"},
				},
			},
		},
		{
			name: "fileTree should not get overwritten by empty string",
			values: orderedData{
				{
					key: "key[options][id]",
					value: []*FileUpload{
						{
							Name: "id1",
						},
					},
				},
				{
					key: "key[options][value]",
					value: []*FileUpload{
						{
							Name: "value1",
						},
					},
				},
				{
					key:   "key[]",
					value: []*FileUpload{},
				},
			},
			wantVal: fileTree{
				"options": fileTree{
					"id":    &FileUpload{Name: "id1"},
					"value": &FileUpload{Name: "value1"},
				},
			},
		},
		{
			name: "there should be error if both fileTree and file upload present #1",
			values: orderedData{
				{
					key: "key[options][id]",
					value: []*FileUpload{
						{
							Name: "id1",
						},
					},
				},
				{
					key: "key[options][value]",
					value: []*FileUpload{
						{
							Name: "value1",
						},
					},
				},
				{
					key: "key",
					value: []*FileUpload{
						{
							Name: "value",
						},
					},
				},
			},
			wantErr: errors.New("invalid multiple values to key 'key' in tree"),
		},
		{
			name: "there should be error if both fileTree and scalar value present #2",
			values: orderedData{
				{
					key: "key",
					value: []*FileUpload{
						{
							Name: "value",
						},
					},
				},
				{
					key: "key[options][id]",
					value: []*FileUpload{
						{
							Name: "id1",
						},
					},
				},
				{
					key: "key[options][value]",
					value: []*FileUpload{
						{
							Name: "value1",
						},
					},
				},
			},
			wantErr: errors.New("invalid multiple values to key 'key' in tree"),
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			var (
				d   = make(fileTree)
				err error
			)

			for _, v := range tt.values {
				err = d.push(v.key, v.value)
				if err != nil {
					break
				}
			}
			if tt.wantErr != nil {
				if err == nil {
					t.Fatalf("want err %+v but got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr.Error()) {
					t.Fatalf("want err %+v but got err %+v", tt.wantErr, err)
				}

				return
			}
			if diff := cmp.Diff(d["key"], tt.wantVal, cmpopts.IgnoreUnexported(FileUpload{})); len(diff) > 0 {
				t.Fatalf("diff should be empty: %+v", diff)
			}
		})
	}
}
