package importpkg

import (
	"strings"

	"github.com/topxeq/xxldb/types"
)

// TypeConverter converts source database types to XxLdb types
type TypeConverter struct{}

// NewTypeConverter creates a new type converter
func NewTypeConverter() *TypeConverter {
	return &TypeConverter{}
}

// ConvertMySQLType converts MySQL type to XxLdb type
func (c *TypeConverter) ConvertMySQLType(mysqlType string) (types.DataType, int) {
	mysqlType = strings.ToUpper(mysqlType)
	mysqlType = strings.TrimRight(mysqlType, " )")

	// Remove length specifier for extraction
	baseType := mysqlType
	if idx := strings.Index(mysqlType, "("); idx > 0 {
		baseType = mysqlType[:idx]
	}

	var length int
	// Extract length if present
	if idx := strings.Index(mysqlType, "("); idx > 0 {
		endIdx := strings.Index(mysqlType, ")")
		if endIdx > idx {
			l := mysqlType[idx+1 : endIdx]
			for _, ch := range l {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
	}

	switch baseType {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "INTEGER", "BIGINT":
		return types.TypeInt, 0
	case "FLOAT", "DOUBLE", "DECIMAL", "NUMERIC", "REAL":
		return types.TypeFloat, 0
	case "CHAR", "CHARACTER", "NCHAR":
		if length == 0 {
			length = 1
		}
		return types.TypeChar, length
	case "VARCHAR", "CHARACTER VARYING", "NVARCHAR", "VARCHAR2":
		if length == 0 {
			length = 255
		}
		return types.TypeVarchar, length
	case "TEXT", "TINYTEXT", "MEDIUMTEXT", "LONGTEXT", "CLOB":
		return types.TypeText, 0
	case "DATE":
		return types.TypeDate, 0
	case "TIME":
		return types.TypeTime, 0
	case "DATETIME", "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE":
		return types.TypeDatetime, 0
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY":
		return types.TypeBlob, 0
	case "BIT", "BOOLEAN", "BOOL":
		return types.TypeInt, 0
	case "YEAR":
		return types.TypeInt, 0
	default:
		return types.TypeVarchar, 255
	}
}

// ConvertPostgreSQLType converts PostgreSQL type to XxLdb type
func (c *TypeConverter) ConvertPostgreSQLType(pgType string) (types.DataType, int) {
	pgType = strings.ToUpper(pgType)
	pgType = strings.TrimRight(pgType, " )")

	// Remove array suffix
	pgType = strings.TrimSuffix(pgType, "[]")

	// Remove length specifier for extraction
	baseType := pgType
	if idx := strings.Index(pgType, "("); idx > 0 {
		baseType = pgType[:idx]
	}

	var length int
	if idx := strings.Index(pgType, "("); idx > 0 {
		endIdx := strings.Index(pgType, ")")
		if endIdx > idx {
			l := pgType[idx+1 : endIdx]
			for _, ch := range l {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
	}

	switch baseType {
	case "SMALLINT", "INTEGER", "INT", "INT4", "INT2", "BIGINT", "INT8", "SERIAL", "BIGSERIAL", "SMALLSERIAL":
		return types.TypeInt, 0
	case "REAL", "FLOAT4", "DOUBLE PRECISION", "FLOAT8", "NUMERIC", "DECIMAL":
		return types.TypeFloat, 0
	case "CHAR", "CHARACTER":
		if length == 0 {
			length = 1
		}
		return types.TypeChar, length
	case "VARCHAR", "CHARACTER VARYING", "TEXT":
		if length == 0 {
			length = 255
		}
		return types.TypeVarchar, length
	case "DATE":
		return types.TypeDate, 0
	case "TIME", "TIME WITHOUT TIME ZONE", "TIME WITH TIME ZONE", "TIMETZ":
		return types.TypeTime, 0
	case "TIMESTAMP", "TIMESTAMP WITHOUT TIME ZONE", "TIMESTAMP WITH TIME ZONE", "TIMESTAMPTZ":
		return types.TypeDatetime, 0
	case "BYTEA":
		return types.TypeBlob, 0
	case "BOOLEAN", "BOOL":
		return types.TypeInt, 0
	case "UUID":
		return types.TypeVarchar, 36
	case "JSON", "JSONB":
		return types.TypeText, 0
	default:
		return types.TypeVarchar, 255
	}
}

// ConvertSQLiteType converts SQLite type to XxLdb type
func (c *TypeConverter) ConvertSQLiteType(sqliteType string) (types.DataType, int) {
	sqliteType = strings.ToUpper(sqliteType)

	var length int
	if idx := strings.Index(sqliteType, "("); idx > 0 {
		endIdx := strings.Index(sqliteType, ")")
		if endIdx > idx {
			l := sqliteType[idx+1 : endIdx]
			for _, ch := range l {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
	}

	// SQLite is flexible with types
	switch {
	case strings.Contains(sqliteType, "INT"):
		return types.TypeInt, 0
	case strings.Contains(sqliteType, "CHAR"):
		if length == 0 {
			length = 255
		}
		return types.TypeChar, length
	case strings.Contains(sqliteType, "CLOB") || strings.Contains(sqliteType, "TEXT"):
		return types.TypeText, 0
	case strings.Contains(sqliteType, "BLOB"):
		return types.TypeBlob, 0
	case strings.Contains(sqliteType, "REAL") || strings.Contains(sqliteType, "FLOA") || strings.Contains(sqliteType, "DOUB"):
		return types.TypeFloat, 0
	case strings.Contains(sqliteType, "DATE"):
		return types.TypeDate, 0
	case strings.Contains(sqliteType, "TIME"):
		return types.TypeDatetime, 0
	case strings.Contains(sqliteType, "VARCHAR"):
		if length == 0 {
			length = 255
		}
		return types.TypeVarchar, length
	default:
		return types.TypeVarchar, 255
	}
}

// ConvertOracleType converts Oracle type to XxLdb type
func (c *TypeConverter) ConvertOracleType(oracleType string) (types.DataType, int) {
	oracleType = strings.ToUpper(oracleType)
	oracleType = strings.TrimRight(oracleType, " )")

	baseType := oracleType
	if idx := strings.Index(oracleType, "("); idx > 0 {
		baseType = oracleType[:idx]
	}

	var length int
	if idx := strings.Index(oracleType, "("); idx > 0 {
		endIdx := strings.Index(oracleType, ")")
		if endIdx > idx {
			l := oracleType[idx+1 : endIdx]
			// Handle NUMBER(precision, scale) format
			if commaIdx := strings.Index(l, ","); commaIdx > 0 {
				l = l[:commaIdx]
			}
			for _, ch := range l {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
	}

	switch baseType {
	case "NUMBER", "INTEGER", "INT", "SMALLINT", "PLS_INTEGER", "BINARY_INTEGER":
		return types.TypeInt, 0
	case "FLOAT", "BINARY_FLOAT", "BINARY_DOUBLE":
		return types.TypeFloat, 0
	case "CHAR", "NCHAR":
		if length == 0 {
			length = 1
		}
		return types.TypeChar, length
	case "VARCHAR", "VARCHAR2", "NVARCHAR", "NVARCHAR2":
		if length == 0 {
			length = 4000
		}
		return types.TypeVarchar, length
	case "CLOB", "NCLOB", "LONG":
		return types.TypeText, 0
	case "DATE":
		return types.TypeDatetime, 0
	case "TIMESTAMP":
		return types.TypeDatetime, 0
	case "BLOB", "RAW", "LONG RAW":
		return types.TypeBlob, 0
	case "ROWID", "UROWID":
		return types.TypeVarchar, 18
	default:
		return types.TypeVarchar, 255
	}
}

// ConvertMSSQLType converts MS SQL Server type to XxLdb type
func (c *TypeConverter) ConvertMSSQLType(mssqlType string) (types.DataType, int) {
	mssqlType = strings.ToUpper(mssqlType)
	mssqlType = strings.TrimRight(mssqlType, " )")

	baseType := mssqlType
	if idx := strings.Index(mssqlType, "("); idx > 0 {
		baseType = mssqlType[:idx]
	}

	var length int
	if idx := strings.Index(mssqlType, "("); idx > 0 {
		endIdx := strings.Index(mssqlType, ")")
		if endIdx > idx {
			l := mssqlType[idx+1 : endIdx]
			// Handle DECIMAL(precision, scale) format
			if commaIdx := strings.Index(l, ","); commaIdx > 0 {
				l = l[:commaIdx]
			}
			for _, ch := range l {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				} else {
					break
				}
			}
		}
	}

	switch baseType {
	case "TINYINT", "SMALLINT", "INT", "INTEGER", "BIGINT":
		return types.TypeInt, 0
	case "REAL", "FLOAT", "DECIMAL", "NUMERIC", "MONEY", "SMALLMONEY":
		return types.TypeFloat, 0
	case "CHAR", "NCHAR":
		if length == 0 {
			length = 1
		}
		return types.TypeChar, length
	case "VARCHAR", "NVARCHAR", "TEXT", "NTEXT":
		if length == 0 || length == -1 { // -1 means MAX in SQL Server
			length = 255
		}
		return types.TypeVarchar, length
	case "DATE":
		return types.TypeDate, 0
	case "TIME":
		return types.TypeTime, 0
	case "DATETIME", "DATETIME2", "SMALLDATETIME", "DATETIMEOFFSET":
		return types.TypeDatetime, 0
	case "BINARY", "VARBINARY", "IMAGE":
		return types.TypeBlob, 0
	case "BIT":
		return types.TypeInt, 0
	case "UNIQUEIDENTIFIER":
		return types.TypeVarchar, 36
	default:
		return types.TypeVarchar, 255
	}
}
