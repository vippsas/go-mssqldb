// +build go1.9

package mssql

import (
	"testing"
	"time"
)

type TestFields struct {
	PBinary       []byte    `tvp:"p_binary"`
	PVarchar      string    `json:"p_varchar"`
	PNvarchar     *string   `json:"p_nvarchar"`
	TimeValue     time.Time `echo:"-"`
	TimeNullValue *time.Time
}

type TestFieldError struct {
	ErrorValue []*byte
}

type TestFieldsUnsupportedTypes struct {
	ErrorType TestFieldError
}

func TestTVPType_columnTypes(t *testing.T) {
	type customTypeAllFieldsSkipOne struct {
		SkipTest int `tvp:"-"`
	}
	type customTypeAllFieldsSkipMoreOne struct {
		SkipTest  int `tvp:"-"`
		SkipTest1 int `json:"-"`
	}
	type skipWrongField struct {
		SkipTest  int
		SkipTest1 []*byte `json:"skip_test" tvp:"-"`
	}
	type structType struct {
		SkipTest  int               `json:"-" tvp:"test"`
		SkipTest1 []*skipWrongField `json:"any" tvp:"tvp"`
	}
	type skipWithAnotherTagValue struct {
		SkipTest int `json:"-" tvp:"test"`
	}

	type fields struct {
		TVPName  string
		TVPValue interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		want    []columnStruct
		wantErr bool
	}{
		{
			name: "Test Pass",
			fields: fields{
				TVPValue: []TestFields{TestFields{}},
			},
		},
		{
			name: "TVPValue has wrong field type",
			fields: fields{
				TVPValue: []TestFieldError{TestFieldError{}},
			},
			wantErr: true,
		},
		{
			name: "TVPValue has wrong type",
			fields: fields{
				TVPValue: []TestFieldsUnsupportedTypes{},
			},
			wantErr: true,
		},
		{
			name: "TVPValue has wrong type",
			fields: fields{
				TVPValue: []structType{},
			},
			wantErr: true,
		},
		{
			name: "CustomTag all fields are skip, single field",
			fields: fields{
				TVPValue: []customTypeAllFieldsSkipOne{},
			},
			wantErr: true,
		},
		{
			name: "CustomTag all fields are skip, > 1 field",
			fields: fields{
				TVPValue: []customTypeAllFieldsSkipMoreOne{},
			},
			wantErr: true,
		},
		{
			name: "CustomTag all fields are skip wrong field type",
			fields: fields{
				TVPValue: []skipWrongField{},
			},
			wantErr: false,
		},
		{
			name: "CustomTag tag value is not -",
			fields: fields{
				TVPValue: []skipWithAnotherTagValue{},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvp := TVPType{
				TVPTypeName: tt.fields.TVPName,
				TVPValue:    tt.fields.TVPValue,
			}
			_, _, err := tvp.columnTypes()
			if (err != nil) != tt.wantErr {
				t.Errorf("TVPType.columnTypes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestTVPType_check(t *testing.T) {
	type fields struct {
		TVPName  string
		TVPValue interface{}
	}

	var nullSlice []*string

	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name:    "TVPTypeName is nil",
			wantErr: true,
		},
		{
			name: "TVPValue is nil",
			fields: fields{
				TVPName:  "Test",
				TVPValue: nil,
			},
			wantErr: true,
		},
		{
			name: "TVPValue is nil",
			fields: fields{
				TVPName: "Test",
			},
			wantErr: true,
		},
		{
			name: "TVPValue isn't slice",
			fields: fields{
				TVPName:  "Test",
				TVPValue: "",
			},
			wantErr: true,
		},
		{
			name: "TVPValue isn't slice",
			fields: fields{
				TVPName:  "Test",
				TVPValue: 12345,
			},
			wantErr: true,
		},
		{
			name: "TVPValue isn't slice",
			fields: fields{
				TVPName:  "Test",
				TVPValue: nullSlice,
			},
			wantErr: true,
		},
		{
			name: "TVPValue isn't right",
			fields: fields{
				TVPName:  "Test",
				TVPValue: []*fields{},
			},
			wantErr: true,
		},
		{
			name: "TVPValue is right",
			fields: fields{
				TVPName:  "Test",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVPValue is right",
			fields: fields{
				TVPName:  "Test",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVPValue is right",
			fields: fields{
				TVPName:  "[Test]",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVPValue is right",
			fields: fields{
				TVPName:  "[123].[Test]",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVP name is right",
			fields: fields{
				TVPName:  "[123].Test",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVP name is right",
			fields: fields{
				TVPName:  "123.[Test]",
				TVPValue: []fields{},
			},
			wantErr: false,
		},
		{
			name: "TVP name is wrong",
			fields: fields{
				TVPName:  "123.[Test\n]",
				TVPValue: []fields{},
			},
			wantErr: true,
		},
		{
			name: "TVP name is wrong",
			fields: fields{
				TVPName:  "123.[Test].456",
				TVPValue: []fields{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvp := TVPType{
				TVPTypeName: tt.fields.TVPName,
				TVPValue:    tt.fields.TVPValue,
			}
			if err := tvp.check(); (err != nil) != tt.wantErr {
				t.Errorf("TVPType.check() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTVPType_encode(t *testing.T) {
	type fields struct {
		TVPTypeName  string
		TVPValue     interface{}
		TVPCustomTag string
	}
	tests := []struct {
		name    string
		fields  fields
		want    []byte
		wantErr bool
	}{
		{
			name: "TVPValue gets error unsupported type",
			fields: fields{
				TVPTypeName: "Test",
				TVPValue:    []TestFieldError{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tvp := TVPType{
				TVPTypeName: tt.fields.TVPTypeName,
				TVPValue:    tt.fields.TVPValue,
			}
			schema, name, err := getSchemeAndName(tt.fields.TVPTypeName)
			if err != nil {
				t.Errorf("getSchemeAndName() error = %v, wantErr %v", err, tt.wantErr)
			}
			_, err = tvp.encode(schema, name)
			if (err != nil) != tt.wantErr {
				t.Errorf("TVPType.encode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func BenchmarkTVPType_check(b *testing.B) {
	type val struct {
		Value string
	}
	tvp := TVPType{
		TVPTypeName: "Test",
		TVPValue:    []val{},
	}
	for i := 0; i < b.N; i++ {
		err := tvp.check()
		if err != nil {
			b.Fail()
		}
	}
}

func BenchmarkColumnTypes(b *testing.B) {
	type str struct {
		bytes      byte
		bytesNull  *byte
		bytesSlice []byte

		int8s      int8
		int8sNull  *int8
		uint8s     uint8
		uint8sNull *uint8

		int16s      int16
		int16sNull  *int16
		uint16s     uint16
		uint16sNull *uint16

		int32s      int32
		int32sNull  *int32
		uint32s     uint32
		uint32sNull *uint32

		int64s      int64
		int64sNull  *int64
		uint64s     uint64
		uint64sNull *uint64

		stringVal     string
		stringValNull *string

		bools     bool
		boolsNull *bool
	}
	wal := make([]str, 100)
	tvp := TVPType{
		TVPTypeName: "Test",
		TVPValue:    wal,
	}
	for i := 0; i < b.N; i++ {
		_, _, err := tvp.columnTypes()
		if err != nil {
			b.Error(err)
		}
	}
}

func TestIsSkipField(t *testing.T) {
	type args struct {
		tvpTagValue    string
		isTvpValue     bool
		jsonTagValue   string
		isJsonTagValue bool
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Empty tags",
			want: false,
		},
		{
			name: "tvp is skip",
			want: true,
			args: args{
				isTvpValue:  true,
				tvpTagValue: skipTagValue,
			},
		},
		{
			name: "tvp is any",
			want: false,
			args: args{
				isTvpValue:  true,
				tvpTagValue: "tvp",
			},
		},
		{
			name: "Json is skip",
			want: true,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   skipTagValue,
			},
		},
		{
			name: "Json is any",
			want: false,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   "any",
			},
		},
		{
			name: "Json is skip tvp is skip",
			want: true,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   skipTagValue,
				isTvpValue:     true,
				tvpTagValue:    skipTagValue,
			},
		},
		{
			name: "Json is skip tvp is any",
			want: false,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   skipTagValue,
				isTvpValue:     true,
				tvpTagValue:    "tvp",
			},
		},
		{
			name: "Json is any tvp is skip",
			want: true,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   "json",
				isTvpValue:     true,
				tvpTagValue:    skipTagValue,
			},
		},
		{
			name: "Json is any tvp is skip",
			want: false,
			args: args{
				isJsonTagValue: true,
				jsonTagValue:   "json",
				isTvpValue:     true,
				tvpTagValue:    "tvp",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSkipField(tt.args.tvpTagValue, tt.args.isTvpValue, tt.args.jsonTagValue, tt.args.isJsonTagValue); got != tt.want {
				t.Errorf("IsSkipField() = %v, schema %v", got, tt.want)
			}
		})
	}
}

func Test_getSchemeAndName(t *testing.T) {
	type args struct {
		tvpName string
	}
	tests := []struct {
		name    string
		args    args
		schema  string
		tvpName string
		wantErr bool
	}{
		{
			name:    "Empty object name",
			wantErr: true,
		},
		{
			name:    "Wrong object name",
			wantErr: true,
			args: args{
				tvpName: "1.2.3",
			},
		},
		{
			name:    "Schema+name",
			wantErr: false,
			args: args{
				tvpName: "obj.tvp",
			},
			schema:  "obj",
			tvpName: "tvp",
		},
		{
			name:    "Schema+name",
			wantErr: false,
			args: args{
				tvpName: "[obj].[tvp]",
			},
			schema:  "obj",
			tvpName: "tvp",
		},
		{
			name:    "only name",
			wantErr: false,
			args: args{
				tvpName: "tvp",
			},
			schema:  "",
			tvpName: "tvp",
		},
		{
			name:    "only name",
			wantErr: false,
			args: args{
				tvpName: "[tvp]",
			},
			schema:  "",
			tvpName: "tvp",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema, name, err := getSchemeAndName(tt.args.tvpName)
			if (err != nil) != tt.wantErr {
				t.Errorf("getSchemeAndName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if schema != tt.schema {
				t.Errorf("getSchemeAndName() schema = %v, schema %v", schema, tt.schema)
			}
			if name != tt.tvpName {
				t.Errorf("getSchemeAndName() name = %v, schema %v", name, tt.tvpName)
			}
		})
	}
}
