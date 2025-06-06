package store

import (
	"context"
	"fmt"

	"github.com/apache/arrow/go/v16/arrow"
	"github.com/apache/arrow/go/v16/arrow/memory"
	"github.com/polarsignals/frostdb"
	"github.com/polarsignals/frostdb/query"
	"github.com/polarsignals/frostdb/query/logicalplan"
)

func Test() (err error) {
	// Create a new column store
	columnstore, err := frostdb.New(frostdb.WithStoragePath("~/tr_db"))
	if err != nil {
		return err
	}
	defer func() {
		err = columnstore.Close()
	}()

	// Open up a database in the column store
	database, _ := columnstore.DB(context.Background(), "simple_db")

	type Simple struct {
		Names map[string]string `frostdb:",asc"`
		Value int64
	}

	table, _ := frostdb.NewGenericTable[Simple](database, "simple_table", memory.DefaultAllocator)

	// Create values to insert into the database these first rows havel dynamic label names of 'firstname' and 'surname'
	frederic := Simple{
		Names: map[string]string{
			"first_name": "Frederic",
			"surname":    "Brancz",
		},
		Value: 100,
	}

	thor := Simple{
		Names: map[string]string{
			"first_name": "Thor",
			"surname":    "Hansen",
		},
		Value: 99,
	}
	_, _ = table.Write(context.Background(), frederic, thor)

	// Now we can insert rows that have middle names into our dynamic column
	matthias := Simple{
		Names: map[string]string{
			"first_name":  "Matthias",
			"middle_name": "Oliver Rainer",
			"surname":     "Loibl",
		},
		Value: 101,
	}
	_, _ = table.Write(context.Background(), matthias)

	// Create a new query engine to retrieve data and print the results
	engine := query.NewEngine(memory.DefaultAllocator, database.TableProvider())
	q := engine.ScanTable("simple_table").
		Project(logicalplan.DynCol("names")). // We don't know all dynamic columns at query time, but we want all of them to be returned.
		Filter(
			logicalplan.Col("names.first_name").Eq(logicalplan.Literal("Frederic")),
		)
	err = q.Execute(context.Background(), func(ctx context.Context, r arrow.Record) error {
		fmt.Println(r)
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
