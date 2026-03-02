package di

import (
	"fmt"
	"log"
	"wspf/app/domain/model"
	"wspf/app/domain/model/config"
	"wspf/app/infra"
	"wspf/app/infra/repository"
	"wspf/app/interface/handler"
	"wspf/app/usecase"

	"github.com/goark/gocli/rwi"
	"go.uber.org/dig"
	"gorm.io/gorm"
)

type digBuilder struct {
	c *dig.Container
}

func NewBuilder() *digBuilder {

	container := dig.New()
	d := &digBuilder{
		c: container,
	}

	setupDefaultDependency(d)

	return d
}

func (d *digBuilder) WithConfig(globalConfig *config.GlobalConfig, serviceConfig *config.ServiceConfig) *digBuilder {
	return d.mustProvide(func() (*config.GlobalConfig, *config.ServiceConfig) {
		return globalConfig, serviceConfig
	})
}

func (d *digBuilder) WithDatabase(globalConfig *config.GlobalConfig) *digBuilder {

	return d.mustProvide(func() (*gorm.DB, error) {

		var (
			conn *gorm.DB
			err  error
		)

		dsn := globalConfig.Database.DSN
		switch globalConfig.Database.Driver {
		case "sqlite":
			conn, err = infra.NewSqliteConnection(dsn, false)
		case "mysql":
			conn, err = infra.NewMysqlConnection(dsn, false)
		default:
			return nil, fmt.Errorf("unsupported database driver: %s", globalConfig.Database.Driver)
		}

		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}

		conn.AutoMigrate(&model.Image{}) //nolint:errcheck

		return conn, nil
	})
}

func (d *digBuilder) WithSQLiteMock() *digBuilder {
	return d.mustProvide(func() (*gorm.DB, error) {
		conn, err := infra.NewSqliteConnection("", false)
		if err != nil {
			return nil, err
		}
		conn.AutoMigrate(&model.Image{}) //nolint:errcheck
		return conn, nil
	})
}

func (d *digBuilder) WithRWI(rwif *rwi.RWI) *digBuilder {
	return d.mustProvide(func() *rwi.RWI {
		return rwif
	})
}

func (d *digBuilder) Build() *dig.Container {
	return d.c
}

func setupDefaultDependency(d *digBuilder) {
	d.mustProvide(repository.NewImageRepositoryImpl)
	d.mustProvide(repository.NewSystemRepositoryImpl)
	d.mustProvide(usecase.NewSystemUsecase)
	d.mustProvide(usecase.NewCatalogUseCase)
	d.mustProvide(handler.NewCatalogHandler)
}

//nolint:unparam
func (d *digBuilder) mustProvide(obj any) *digBuilder {
	if err := d.c.Provide(obj); err != nil {
		// A Provide failure indicates a DI configuration bug; Fatal because startup cannot continue.
		log.Fatalf("DI provider registration failed: %v", err)
	}
	return d
}
