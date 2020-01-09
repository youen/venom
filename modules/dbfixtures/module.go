package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/mitchellh/mapstructure"
	migrate "github.com/rubenv/sql-migrate"
	fixtures "gopkg.in/testfixtures.v2"
	// SQL drivers.
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"

	"github.com/ovh/venom"
	"github.com/ovh/venom/lib/module"
)

// Module is a venom executor that can load
// fixtures in many databases, using YAML schemas.
type Module struct {
	Files      []string `json:"files" yaml:"files"`
	Folder     string   `json:"folder" yaml:"folder"`
	Database   string   `json:"database" yaml:"database"`
	DSN        string   `json:"dsn" yaml:"dsn"`
	Schemas    []string `json:"schemas" yaml:"schemas"`
	Migrations string   `json:"migrations" yaml:"migrations"`
}

// Result represents a step result.
type Result struct {
	Module Module `json:"module,omitempty" yaml:"module,omitempty"`
}

func (e Module) Manifest() venom.VenomModuleManifest {
	return venom.VenomModuleManifest{
		Name:    "dbfixtures",
		Type:    "moduleCommand",
		Version: venom.Version,
	}
}

// Run execute TestStep of type exec
func (e Module) Run(ctx venom.TestContext, step venom.TestStep) (venom.ModuleResult, error) {
	// Transform step to Executor instance.
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}
	// Connect to the database and ping it.
	module.Debugf("connecting to database %s, %s\n", e.Database, e.DSN)

	db, err := sql.Open(e.Database, e.DSN)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}
	defer db.Close()

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	workdir, _ := os.Getwd()

	// Load and import the schemas in the database
	// if the argument is specified.
	if len(e.Schemas) != 0 {
		for _, s := range e.Schemas {
			module.Debugf("loading schema from file %s\n", s)
			s = path.Join(workdir, s)
			sbytes, errs := ioutil.ReadFile(s)
			if errs != nil {
				return nil, errs
			}
			if _, err = db.Exec(string(sbytes)); err != nil {
				return nil, fmt.Errorf("failed to exec schema from file %s : %v", s, err)
			}
		}
	} else if e.Migrations != "" {
		module.Debugf("loading migrations from folder %s\n", e.Migrations)

		dir := path.Join(workdir, e.Migrations)
		migrations := &migrate.FileMigrationSource{
			Dir: dir,
		}
		n, errMigrate := migrate.Exec(db, e.Database, migrations, migrate.Up)
		if errMigrate != nil {
			return nil, fmt.Errorf("failed to apply up migrations: %s", errMigrate)
		}
		module.Debugf("applied %d migrations\n", n)
	}
	// Load fixtures in the databases.
	// Bu default the package refuse to load if the database
	// does not contains test to avoid wiping a production db.
	fixtures.SkipDatabaseNameCheck(true)
	if err = loadFixtures(db, e.Files, e.Folder, databaseHelper(e.Database), workdir); err != nil {
		return nil, err
	}
	r := Result{Module: e}

	return venom.Dump(r)
}

// ZeroValueResult return an empty implemtation of this executor result
func (Module) ZeroValueResult() venom.ModuleResult {
	r, _ := venom.Dump(Result{})
	return r
}

// GetDefaultAssertions return the default assertions of the module.
func (e Module) GetDefaultAssertions() venom.StepAssertions {
	return venom.StepAssertions{Assertions: []string{}}
}

// loadFixtures loads the fixtures in the database.
// It gives priority to the fixtures files found in folder,
// and switch to the list of files if no folder was specified.
func loadFixtures(db *sql.DB, files []string, folder string, helper fixtures.Helper, workdir string) error {
	if folder != "" {
		module.Debugf("loading fixtures from folder %s\n", path.Join(workdir, folder))

		c, err := fixtures.NewFolder(db, helper, path.Join(workdir, folder))
		if err != nil {
			return fmt.Errorf("failed to create folder context: %v", err)
		}
		if err = c.Load(); err != nil {
			return fmt.Errorf("failed to load fixtures from folder %s: %v", path.Join(workdir, folder), err)
		}
		return nil
	}
	if len(files) != 0 {
		module.Debugf("loading fixtures from files: %v\n", files)
		for i := range files {
			files[i] = path.Join(workdir, files[i])
		}
		c, err := fixtures.NewFiles(db, helper, files...)
		if err != nil {
			return fmt.Errorf("failed to create files context: %v", err)
		}
		if err = c.Load(); err != nil {
			return fmt.Errorf("failed to load fixtures from files: %v", err)
		}
		return nil
	}
	module.Debugf("neither files or folder parameter was used\n")

	return nil
}

func databaseHelper(name string) fixtures.Helper {
	switch name {
	case "postgres":
		return &fixtures.PostgreSQL{}
	case "mysql":
		return &fixtures.MySQL{}
	}
	return nil
}
