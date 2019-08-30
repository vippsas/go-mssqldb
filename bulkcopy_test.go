// +build go1.9

package mssql

import (
	"context"
	"database/sql"
	"encoding/hex"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBulkcopy(t *testing.T) {
	// TDS level Bulk Insert is not supported on Azure SQL Server.
	if dsn := makeConnStr(t); strings.HasSuffix(strings.Split(dsn.Host, ":")[0], ".database.windows.net") {
		t.Skip("TDS level bulk copy is not supported on Azure SQL Server")
	}
	type testValue struct {
		colname string
		val     interface{}
	}

	type differentExpected struct {
		input    interface{}
		expected interface{}
	}

	tableName := "#table_test"
	geom, _ := hex.DecodeString("E6100000010C00000000000034400000000000004440")
	bin, _ := hex.DecodeString("ba8b7782168d4033a299333aec17bd33")
	testValues := []testValue{

		{"test_nvarchar", "ab©ĎéⒻghïjklmnopqЯ☀tuvwxyz"},
		{"test_nvarchar_4000", strings.Repeat("Ⓕ", 4000)}, // edge case
		{"test_nvarchar_max", strings.Repeat("Ⓕ", 4001)},
		{"test_varchar", "abcdefg"},
		{"test_varchar_8000", strings.Repeat("a", 8000)}, // edge case
		{"test_varchar_max", strings.Repeat("a", 8001)},
		{"test_char", "abcdefg   "},
		{"test_nchar", "abcdefg   "},
		{"test_text", "abcdefg"},
		{"test_ntext", "abcdefg"},
		{"test_float", 1234.56},
		{"test_floatn", 1234.56},
		{"test_real", 1234.56},
		{"test_realn", 1234.56},
		{"test_bit", true},
		{"test_bitn", nil},
		{"test_smalldatetime", time.Date(2010, 11, 12, 13, 14, 0, 0, time.UTC)},
		{"test_smalldatetimen", time.Date(2010, 11, 12, 13, 14, 0, 0, time.UTC)},
		{"test_datetime", time.Date(2010, 11, 12, 13, 14, 15, 120000000, time.UTC)},
		{"test_datetimen", time.Date(2010, 11, 12, 13, 14, 15, 120000000, time.UTC)},
		{"test_datetimen_1", time.Date(4010, 11, 12, 13, 14, 15, 120000000, time.UTC)},
		{"test_datetime2_1", time.Date(2010, 11, 12, 13, 14, 15, 0, time.UTC)},
		{"test_datetime2_3", time.Date(2010, 11, 12, 13, 14, 15, 123000000, time.UTC)},
		{"test_datetime2_7", time.Date(2010, 11, 12, 13, 14, 15, 123000000, time.UTC)},
		{"test_date", time.Date(2010, 11, 12, 00, 00, 00, 0, time.UTC)},
		{"test_tinyint", 255},
		{"test_smallint", 32767},
		{"test_smallintn", nil},
		{"test_int", 2147483647},
		{"test_bigint", 9223372036854775807},
		{"test_bigintn", nil},
		{"test_geom", geom},
		{"test_uniqueidentifier", []byte{0x6F, 0x96, 0x19, 0xFF, 0x8B, 0x86, 0xD0, 0x11, 0xB4, 0x2D, 0x00, 0xC0, 0x4F, 0xC9, 0x64, 0xFF}},
		// {"test_smallmoney", 1234.56},
		{"test_decimal_18_0", 1234.0001},
		{"test_decimal_9_2", 1234.560001},
		{"test_decimal_20_0", 1234.0001},
		{"test_numeric_30_10", 1234567.1234567},
		{"test_varbinary", []byte("1")},
		{"test_varbinary_16", bin},
		{"test_varbinary_max", bin},
		{"test_binary", []byte("1")},
		{"test_binary_16", bin},

		// money must be input as int64 to bulk insert, but scans back as a string on SELECT, so use `differentExpected` to provide
		// different input and expected output

		// First test: We do some byte shuffling for the money type, so make sure every byte is unique in the test.
		{"test_money_1", differentExpected{
			int64(-(0x01<<56 | 0x02<<48 | 0x03<<40 | 0x04<<32 | 0x05<<24 | 0x06<<16 | 0x07<<8 | 0x08)), // evaluates to 72623859790382856
			[]byte("-7262385979038.2856")}},
		// maximum positive, minimum negative, and zero values
		{"test_money_2", differentExpected{math.MaxInt64, []byte("922337203685477.5807")}},
		{"test_money_3", differentExpected{math.MinInt64, []byte("-922337203685477.5808")}},
		{"test_money_4", differentExpected{0, []byte("0.0000")}},

		{"test_money_n_1", differentExpected{
			int64(-(0x01<<56 | 0x02<<48 | 0x03<<40 | 0x04<<32 | 0x05<<24 | 0x06<<16 | 0x07<<8 | 0x08)), // evaluates to 72623859790382856
			[]byte("-7262385979038.2856")}},
		// maximum positive, minimum negative, and zero values
		{"test_money_n_2", differentExpected{math.MaxInt64, []byte("922337203685477.5807")}},
		{"test_money_n_3", differentExpected{math.MinInt64, []byte("-922337203685477.5808")}},
		{"test_money_n_4", differentExpected{0, []byte("0.0000")}},
		{"test_money_n_5", nil},
	}

	columns := make([]string, len(testValues))
	for i, val := range testValues {
		columns[i] = val.colname
	}

	values := make([]interface{}, len(testValues))
	for i, val := range testValues {
		switch t := val.val.(type) {
		case differentExpected:
			values[i] = t.input
		default:
			values[i] = val.val
		}
	}

	pool := open(t)
	defer pool.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Now that session resetting is supported, the use of the per session
	// temp table requires the use of a dedicated connection from the connection
	// pool.
	conn, err := pool.Conn(ctx)
	if err != nil {
		t.Fatal("failed to pull connection from pool", err)
	}
	defer conn.Close()

	err = setupTable(ctx, t, conn, tableName)
	if err != nil {
		t.Error("Setup table failed: ", err)
		return
	}

	t.Log("Preparing copy in statement")

	stmt, err := conn.PrepareContext(ctx, CopyIn(tableName, BulkOptions{}, columns...))

	for i := 0; i < 10; i++ {
		t.Logf("Executing copy in statement %d time with %d values", i+1, len(values))
		_, err = stmt.Exec(values...)
		if err != nil {
			t.Error("AddRow failed: ", err.Error())
			return
		}
	}

	result, err := stmt.Exec()
	if err != nil {
		t.Fatal("bulkcopy failed: ", err.Error())
	}

	insertedRowCount, _ := result.RowsAffected()
	if insertedRowCount == 0 {
		t.Fatal("0 row inserted!")
	}

	//check that all rows are present
	var rowCount int
	err = conn.QueryRowContext(ctx, "select count(*) c from "+tableName).Scan(&rowCount)

	if rowCount != 10 {
		t.Errorf("unexpected row count %d", rowCount)
	}

	//data verification
	rows, err := conn.QueryContext(ctx, "select "+strings.Join(columns, ",")+" from "+tableName)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {

		ptrs := make([]interface{}, len(columns))
		container := make([]interface{}, len(columns))
		for i, _ := range ptrs {
			ptrs[i] = &container[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		for i, c := range testValues {
			var expected interface{}
			switch t := c.val.(type) {
			case differentExpected:
				expected = t.expected
			default:
				expected = c.val
			}
			if !compareValue(container[i], expected) {
				t.Errorf("columns %s : expected: %v, got: %v\n", c.colname, string(expected.([]byte)), string(container[i].([]byte)))
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Error(err)
	}
}

func compareValue(a interface{}, expected interface{}) bool {
	switch expected := expected.(type) {
	case int:
		return int64(expected) == a
	case int32:
		return int64(expected) == a
	case int64:
		return int64(expected) == a
	case float64:
		if got, ok := a.([]uint8); ok {
			var nf sql.NullFloat64
			nf.Scan(got)
			a = nf.Float64
		}
		return math.Abs(expected-a.(float64)) < 0.0001
	default:
		return reflect.DeepEqual(expected, a)
	}
}

func setupTable(ctx context.Context, t *testing.T, conn *sql.Conn, tableName string) (err error) {
	tablesql := `CREATE TABLE ` + tableName + ` (
	[id] [int] IDENTITY(1,1) NOT NULL,
	[test_nvarchar] [nvarchar](50) NULL,
	[test_nvarchar_4000] [nvarchar](4000) NULL,
	[test_nvarchar_max] [nvarchar](max) NULL,
	[test_varchar] [varchar](50) NULL,
	[test_varchar_8000] [varchar](8000) NULL,
	[test_varchar_max] [varchar](max) NULL,
	[test_char] [char](10) NULL,
	[test_nchar] [nchar](10) NULL,
	[test_text] [text] NULL,
	[test_ntext] [ntext] NULL,
	[test_float] [float] NOT NULL,
	[test_floatn] [float] NULL,
	[test_real] [real] NULL,
	[test_realn] [real] NULL,
	[test_bit] [bit] NOT NULL,
	[test_bitn] [bit] NULL,
	[test_smalldatetime] [smalldatetime] NOT NULL,
	[test_smalldatetimen] [smalldatetime] NULL,
	[test_datetime] [datetime] NOT NULL,
	[test_datetimen] [datetime] NULL,
	[test_datetimen_1] [datetime] NULL,
	[test_datetime2_1] [datetime2](1) NULL,
	[test_datetime2_3] [datetime2](3) NULL,
	[test_datetime2_7] [datetime2](7) NULL,
	[test_date] [date] NULL,
	[test_tinyint] [tinyint] NULL,
	[test_smallint] [smallint] NOT NULL,
	[test_smallintn] [smallint] NULL,
	[test_int] [int] NULL,
	[test_bigint] [bigint] NOT NULL,
	[test_bigintn] [bigint] NULL,
	[test_geom] [geometry] NULL,
	[test_geog] [geography] NULL,
	[text_xml] [xml] NULL,
	[test_uniqueidentifier] [uniqueidentifier] NULL,
	[test_decimal_18_0] [decimal](18, 0) NULL,
	[test_decimal_9_2] [decimal](9, 2) NULL,
	[test_decimal_20_0] [decimal](20, 0) NULL,
	[test_numeric_30_10] [decimal](30, 10) NULL,
	[test_varbinary] VARBINARY NOT NULL,
	[test_varbinary_16] VARBINARY(16) NOT NULL,
	[test_varbinary_max] VARBINARY(max) NOT NULL,
	[test_binary] BINARY NOT NULL,
	[test_binary_16] BINARY(16) NOT NULL,
	[test_money_1] MONEY NOT NULL,
	[test_money_2] MONEY NOT NULL,
	[test_money_3] MONEY NOT NULL,
	[test_money_4] MONEY NOT NULL,
	[test_money_n_1] MONEY NULL,
	[test_money_n_2] MONEY NULL,
	[test_money_n_3] MONEY NULL,
	[test_money_n_4] MONEY NULL,
	[test_money_n_5] MONEY NULL
 CONSTRAINT [PK_` + tableName + `_id] PRIMARY KEY CLUSTERED 
(
	[id] ASC
)WITH (PAD_INDEX = OFF, STATISTICS_NORECOMPUTE = OFF, IGNORE_DUP_KEY = OFF, ALLOW_ROW_LOCKS = ON, ALLOW_PAGE_LOCKS = ON) ON [PRIMARY]
) ON [PRIMARY] TEXTIMAGE_ON [PRIMARY];`
	_, err = conn.ExecContext(ctx, tablesql)
	if err != nil {
		t.Fatal("tablesql failed:", err)
	}
	return
}
