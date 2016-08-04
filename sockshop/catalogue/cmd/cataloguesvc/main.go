package main

import (
	"database/sql"
	// "encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kit/kit/log"

	"net/http"

	// "sort"

	_ "github.com/go-sql-driver/mysql"
	"github.com/weaveworks/weaveDemo/catalogue"
	"golang.org/x/net/context"
)

func main() {
	var (
		port   = flag.String("port", "8081", "Port to bind HTTP listener") // TODO(pb): should be -addr, default ":8081"
		images = flag.String("images", "./images/", "Image path")
		dbName = flag.String("db", "socksdb", "Database name")
	)
	flag.Parse()

	// Mechanical stuff.
	errc := make(chan error)
	ctx := context.Background()

	// Log domain.
	var logger log.Logger
	{
		logger = log.NewLogfmtLogger(os.Stderr)
		logger = log.NewContext(logger).With("ts", log.DefaultTimestampUTC)
		logger = log.NewContext(logger).With("caller", log.DefaultCaller)
	}

	// Data domain.
	// TODO pull user/password from env?
	db, err := sql.Open("mysql", "catalogue_user:default_password@tcp(catalogue-db:3306)/"+*dbName)
	if err != nil {
		logger.Log("err", err)
		// TODO should we exit if not DB?
		os.Exit(1)
	}
	defer db.Close()

	// Check if DB connection can be made, only for logging purposes, should not fail/exit
	err = db.Ping()
	if err != nil {
		logger.Log("Error", "Unable to connect to Database", "DB", dbName)
	}

	// Service domain.
	var service catalogue.Service
	{
		service = catalogue.NewFixedService(db)
		service = catalogue.LoggingMiddleware(logger)(service)
	}

	// Endpoint domain.
	endpoints := catalogue.MakeEndpoints(service)

	// Create and launch the HTTP server.
	go func() {
		logger.Log("transport", "HTTP", "port", *port)
		handler := catalogue.MakeHTTPHandler(ctx, endpoints, *images, logger)
		errc <- http.ListenAndServe(":"+*port, handler)
	}()

	// Capture interrupts.
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errc <- fmt.Errorf("%s", <-c)
	}()

	logger.Log("exit", <-errc)
}
