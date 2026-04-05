package importpkg

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/sijms/go-ora/v2"
)

// OracleImporter imports data from Oracle
type OracleImporter struct {
	db        *sql.DB
	dsn       string
	converter *TypeConverter
}

// NewOracleImporter creates a new Oracle importer
func NewOracleImporter() *OracleImporter {
	return &OracleImporter{
		converter: NewTypeConverter(),
	}
}

// Connect connects to the Oracle database
func (o *OracleImporter) Connect(dsn string) error {
	// Oracle DSN format: oracle://user:password@host:port/service_name
	// Or: user/password@host:port/service_name

	var connStr string
	// If it's a URL format, use it directly
	if len(dsn) > 8 && dsn[:8] == "oracle://" {
		connStr = dsn
	} else {
		// Assume it's a connection string format, build URL
		connStr = "oracle://" + dsn
	}

	db, err := sql.Open("oracle", connStr)
	if err != nil {
		return fmt.Errorf("failed to open Oracle connection: %w", err)
	}

	if err := db.Ping(); err != nil {
		db.Close()
		return fmt.Errorf("failed to ping Oracle: %w", err)
	}

	o.db = db
	o.dsn = dsn
	return nil
}

// Disconnect closes the connection
func (o *OracleImporter) Disconnect() error {
	if o.db != nil {
		return o.db.Close()
	}
	return nil
}

// ListTables lists all tables in the database
func (o *OracleImporter) ListTables(dbName string) ([]string, error) {
	query := `
		SELECT TABLE_NAME
		FROM USER_TABLES
		ORDER BY TABLE_NAME`

	rows, err := o.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tables = append(tables, name)
	}

	return tables, nil
}

// GetTableSchema gets the schema of a table including constraints
func (o *OracleImporter) GetTableSchema(dbName, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
	}

	// Get columns
	if err := o.getColumns(tableName, schema); err != nil {
		return nil, err
	}

	// Get primary keys
	if err := o.getPrimaryKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get foreign keys
	if err := o.getForeignKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get unique constraints
	if err := o.getUniqueKeys(tableName, schema); err != nil {
		return nil, err
	}

	// Get check constraints
	if err := o.getCheckConstraints(tableName, schema); err != nil {
		return nil, err
	}

	// Get indexes
	if err := o.getIndexes(tableName, schema); err != nil {
		return nil, err
	}

	return schema, nil
}

// getColumns retrieves column information
func (o *OracleImporter) getColumns(tableName string, schema *TableSchema) error {
	query := `
		SELECT
			COLUMN_NAME,
			DATA_TYPE,
			DATA_LENGTH,
			NULLABLE,
			DATA_DEFAULT
		FROM USER_TAB_COLUMNS
		WHERE TABLE_NAME = UPPER(:1)
		ORDER BY COLUMN_ID`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return fmt.Errorf("failed to get table schema: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, colType, isNullable string
		var dataLength sql.NullInt64
		var defaultValue sql.NullString

		if err := rows.Scan(&name, &colType, &dataLength, &isNullable, &defaultValue); err != nil {
			return err
		}

		// Combine type with length for some types
		fullType := colType
		if dataLength.Valid && (strings.ToUpper(colType) == "VARCHAR2" || strings.ToUpper(colType) == "CHAR") {
			fullType = fmt.Sprintf("%s(%d)", colType, dataLength.Int64)
		}

		targetType, length := o.converter.ConvertOracleType(fullType)

		col := ColumnInfo{
			Name:       name,
			SourceType: fullType,
			TargetType: targetType,
			Length:     length,
			IsNullable: isNullable == "Y",
		}

		if defaultValue.Valid {
			col.DefaultValue = o.parseDefaultValue(defaultValue.String)
		}

		schema.Columns = append(schema.Columns, col)
	}

	return nil
}

// getPrimaryKeys retrieves primary key constraint
func (o *OracleImporter) getPrimaryKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT UCC.COLUMN_NAME, UC.CONSTRAINT_NAME
		FROM USER_CONSTRAINTS UC
		JOIN USER_CONS_COLUMNS UCC ON UC.CONSTRAINT_NAME = UCC.CONSTRAINT_NAME
		WHERE UC.CONSTRAINT_TYPE = 'P'
		AND UC.TABLE_NAME = UPPER(:1)
		ORDER BY UCC.POSITION`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var columnName, constraintName string
		if err := rows.Scan(&columnName, &constraintName); err != nil {
			return err
		}
		schema.PrimaryKeys = append(schema.PrimaryKeys, columnName)

		// Mark column as primary key
		for i, col := range schema.Columns {
			if strings.EqualFold(col.Name, columnName) {
				schema.Columns[i].IsPrimaryKey = true
			}
		}
	}

	return nil
}

// getForeignKeys retrieves foreign key constraints
func (o *OracleImporter) getForeignKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT
			UC.CONSTRAINT_NAME,
			UCC.COLUMN_NAME,
			UC.R_CONSTRAINT_NAME,
			AC.TABLE_NAME AS REF_TABLE,
			ACC.COLUMN_NAME AS REF_COLUMN,
			RC.DELETE_RULE,
			RC.UPDATE_RULE
		FROM USER_CONSTRAINTS UC
		JOIN USER_CONS_COLUMNS UCC ON UC.CONSTRAINT_NAME = UCC.CONSTRAINT_NAME
		JOIN USER_CONSTRAINTS RC ON UC.R_CONSTRAINT_NAME = RC.CONSTRAINT_NAME
		JOIN USER_CONS_COLUMNS ACC ON RC.CONSTRAINT_NAME = ACC.CONSTRAINT_NAME
		JOIN USER_TABLES AC ON RC.TABLE_NAME = AC.TABLE_NAME
		WHERE UC.CONSTRAINT_TYPE = 'R'
		AND UC.TABLE_NAME = UPPER(:1)
		ORDER BY UC.CONSTRAINT_NAME, UCC.POSITION`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	fkMap := make(map[string]*ForeignKeyInfo)
	for rows.Next() {
		var constraintName, columnName, rConstraintName, refTable, refColumn string
		var deleteRule sql.NullString
		var updateRule sql.NullString

		if err := rows.Scan(&constraintName, &columnName, &rConstraintName, &refTable, &refColumn, &deleteRule, &updateRule); err != nil {
			return err
		}

		if fk, exists := fkMap[constraintName]; exists {
			fk.Columns = append(fk.Columns, columnName)
			fk.RefColumns = append(fk.RefColumns, refColumn)
		} else {
			fk := &ForeignKeyInfo{
				Name:       constraintName,
				Columns:    []string{columnName},
				RefTable:   refTable,
				RefColumns: []string{refColumn},
			}
			if deleteRule.Valid {
				fk.OnDelete = deleteRule.String
			}
			if updateRule.Valid {
				fk.OnUpdate = updateRule.String
			}
			fkMap[constraintName] = fk
		}
	}

	for _, fk := range fkMap {
		schema.ForeignKeys = append(schema.ForeignKeys, *fk)
	}

	return nil
}

// getUniqueKeys retrieves unique constraints
func (o *OracleImporter) getUniqueKeys(tableName string, schema *TableSchema) error {
	query := `
		SELECT UC.CONSTRAINT_NAME, UCC.COLUMN_NAME
		FROM USER_CONSTRAINTS UC
		JOIN USER_CONS_COLUMNS UCC ON UC.CONSTRAINT_NAME = UCC.CONSTRAINT_NAME
		WHERE UC.CONSTRAINT_TYPE = 'U'
		AND UC.TABLE_NAME = UPPER(:1)
		ORDER BY UC.CONSTRAINT_NAME, UCC.POSITION`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	ukMap := make(map[string]*UniqueKeyInfo)
	for rows.Next() {
		var constraintName, columnName string
		if err := rows.Scan(&constraintName, &columnName); err != nil {
			return err
		}

		if uk, exists := ukMap[constraintName]; exists {
			uk.Columns = append(uk.Columns, columnName)
		} else {
			ukMap[constraintName] = &UniqueKeyInfo{
				Name:    constraintName,
				Columns: []string{columnName},
			}
		}
	}

	for _, uk := range ukMap {
		schema.UniqueKeys = append(schema.UniqueKeys, *uk)

		// Mark single-column unique
		if len(uk.Columns) == 1 {
			for i, col := range schema.Columns {
				if strings.EqualFold(col.Name, uk.Columns[0]) {
					schema.Columns[i].IsUnique = true
				}
			}
		}
	}

	return nil
}

// getCheckConstraints retrieves check constraints
func (o *OracleImporter) getCheckConstraints(tableName string, schema *TableSchema) error {
	query := `
		SELECT CONSTRAINT_NAME, SEARCH_CONDITION
		FROM USER_CONSTRAINTS
		WHERE CONSTRAINT_TYPE = 'C'
		AND TABLE_NAME = UPPER(:1)
		AND GENERATED = 'USER NAME'`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var condition sql.NullString
		if err := rows.Scan(&name, &condition); err != nil {
			return err
		}

		if condition.Valid {
			schema.CheckConstraints = append(schema.CheckConstraints, CheckInfo{
				Name:       name,
				Definition: condition.String,
			})
		}
	}

	return nil
}

// getIndexes retrieves index information
func (o *OracleImporter) getIndexes(tableName string, schema *TableSchema) error {
	query := `
		SELECT I.INDEX_NAME, IC.COLUMN_NAME, I.UNIQUENESS
		FROM USER_INDEXES I
		JOIN USER_IND_COLUMNS IC ON I.INDEX_NAME = IC.INDEX_NAME
		WHERE I.TABLE_NAME = UPPER(:1)
		AND I.INDEX_NAME NOT IN (
			SELECT CONSTRAINT_NAME FROM USER_CONSTRAINTS
			WHERE CONSTRAINT_TYPE IN ('P', 'U')
		)
		ORDER BY I.INDEX_NAME, IC.COLUMN_POSITION`

	rows, err := o.db.Query(query, tableName)
	if err != nil {
		return err
	}
	defer rows.Close()

	idxMap := make(map[string]*IndexInfo)
	for rows.Next() {
		var indexName, columnName, uniqueness string
		if err := rows.Scan(&indexName, &columnName, &uniqueness); err != nil {
			return err
		}

		if idx, exists := idxMap[indexName]; exists {
			idx.Columns = append(idx.Columns, columnName)
		} else {
			idxMap[indexName] = &IndexInfo{
				Name:    indexName,
				Columns: []string{columnName},
				Unique:  uniqueness == "UNIQUE",
			}
		}
	}

	for _, idx := range idxMap {
		schema.Indexes = append(schema.Indexes, *idx)
	}

	return nil
}

// parseDefaultValue parses Oracle default value
func (o *OracleImporter) parseDefaultValue(val string) interface{} {
	if val == "" {
		return nil
	}
	// Remove quotes if present
	if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
		return val[1 : len(val)-1]
	}
	return val
}

// ReadTable reads rows from a table
func (o *OracleImporter) ReadTable(dbName, tableName string, offset, limit int) ([][]interface{}, error) {
	// Oracle uses ROWNUM for pagination
	query := fmt.Sprintf(`
		SELECT * FROM (
			SELECT a.*, ROWNUM rn FROM (
				SELECT * FROM %s
			) a WHERE ROWNUM <= %d
		) WHERE rn > %d`, o.quoteIdentifier(tableName), offset+limit, offset)

	rows, err := o.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to read table: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// Remove the 'rn' column that we added
	if len(columns) > 0 {
		columns = columns[:len(columns)-1]
	}

	var results [][]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns)+1) // +1 for rn column
		valuePtrs := make([]interface{}, len(columns)+1)
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		// Remove the rn column value
		rowValues := values[:len(columns)]

		// Convert []byte to string for text columns
		for i, v := range rowValues {
			if b, ok := v.([]byte); ok {
				rowValues[i] = string(b)
			}
		}

		results = append(results, rowValues)
	}

	return results, nil
}

// ReadTableCount gets the total number of rows in a table
func (o *OracleImporter) ReadTableCount(dbName, tableName string) (int64, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", o.quoteIdentifier(tableName))

	var count int64
	if err := o.db.QueryRow(query).Scan(&count); err != nil {
		return 0, fmt.Errorf("failed to count rows: %w", err)
	}

	return count, nil
}

// quoteIdentifier quotes an Oracle identifier
func (o *OracleImporter) quoteIdentifier(name string) string {
	return fmt.Sprintf("\"%s\"", strings.ToUpper(name))
}
