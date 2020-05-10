package main

import (
	"github.com/thomasvvugt/fiber-boilerplate/app/configuration"
	"github.com/thomasvvugt/fiber-boilerplate/app/models"
	"github.com/thomasvvugt/fiber-boilerplate/app/providers"
	"github.com/thomasvvugt/fiber-boilerplate/database"
	"github.com/thomasvvugt/fiber-boilerplate/routes"

	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/gofiber/compression"
	"github.com/gofiber/cors"
	"github.com/gofiber/fiber"
	"github.com/gofiber/helmet"
	"github.com/gofiber/logger"
	"github.com/gofiber/recover"
)

func main() {
	// Load configurations
	config, err := configuration.LoadConfigurations()
	if err != nil {
		// Error when loading the configurations
		log.Fatalf("An error occurred while loading the configurations: %v", err)
	}

	// Create a new Fiber application
	app := fiber.New(&config.Fiber)

	// Use the Logger Middleware if enabled
	if config.Enabled["logger"] {
		app.Use(logger.New(config.Logger))
	}

	// Use the Recover Middleware if enabled
	if config.Enabled["recover"] {
		app.Use(recover.New(config.Recover))
	}

	// Set the Template Middleware configuration
	if config.Enabled["template"] {
		app.Settings.TemplateEngine = config.TemplateEngine
	}

	// Use HTTP best practices
	app.Use(func(c *fiber.Ctx) {
		// Suppress the `www.` at the beginning of URLs
		if config.App.SuppressWWW {
			providers.SuppressWWW(c)
		}
		// Force HTTPS protocol
		if config.App.ForceHTTPS {
			providers.ForceHTTPS(c)
		}
		// Move on the the next route
		c.Next()
	})

	// Use the Compression Middleware if enabled
	if config.Enabled["compression"] {
		app.Use(compression.New(config.Compression))
	}

	// Use the CORS Middleware if enabled
	if config.Enabled["cors"] {
		app.Use(cors.New(config.CORS))
	}

	// Use the Helmet Middleware if enabled
	if config.Enabled["helmet"] {
		app.Use(helmet.New(config.Helmet))
	}

	// Connect to a database
	if config.Enabled["database"] {
		database.Connect(&config.Database)
	}

	// Run auto migrations
	database.Instance().AutoMigrate(&models.Role{})
	database.Instance().AutoMigrate(&models.User{})
	// Set CASCADE foreign key
	database.Instance().Model(&models.User{}).AddForeignKey("role_id", "roles(id)", "RESTRICT", "CASCADE")

	// Register application web routes
	routes.RegisterWeb(app)

	// Register application API routes (using the /api/v1 group)
	api := app.Group("/api")
	apiv1 := api.Group("/v1")
	routes.RegisterAPI(apiv1)

	// Serve public, static files
	if config.Enabled["public"] {
		app.Static(config.PublicPrefix, config.PublicRoot, config.Public)
	}

	// Custom 404-page
	app.Use(func(c *fiber.Ctx) {
		c.SendStatus(404)
		if err := c.Render("errors/404", fiber.Map{}); err != nil {
			c.Status(500).Send(err.Error())
		}
	})

	// Close any connections on interrupt signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		exit(&config, app, nil)
	}()

	// Start listening on the specified address
	err = app.Listen(config.App.Listen)
	if err != nil {
		// Exit the application
		exit(&config, app, err)
	}
}

func exit(config *configuration.Configuration, app *fiber.App, err error) {
	// Close database connection
	var dbErr error
	if config.Enabled["database"] {
		dbErr = database.Close()
		if dbErr != nil {
			fmt.Printf("Closed database: %v\n", dbErr)
		} else {
			fmt.Println("Closed database.")
		}
	}
	// Shutdown Fiber application
	var appErr error
	if err != nil {
		fmt.Printf("Shutdown Fiber application: %v", err)
		appErr = err
	} else {
		appErr = app.Shutdown()
		if appErr != nil {
			fmt.Printf("Shutdown Fiber application: %v", appErr)
		} else {
			fmt.Print("Shutdown Fiber application.")
		}
	}
	// Return with corresponding exit code
	if dbErr != nil || appErr != nil {
		os.Exit(1)
	} else {
		os.Exit(0)
	}
}
