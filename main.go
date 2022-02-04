package main

import (
	"github.com/djumanoff/amqp"
	"github.com/gin-gonic/gin"
	setdata_common "github.com/kirigaikabuto/setdata-common"
	tik_api_lib "github.com/kirigaikabuto/tik-api-lib"
	token_str "github.com/kirigaikabuto/tik-api-lib/auth"
	"github.com/spf13/viper"
	"github.com/urfave/cli"
	"log"
	"os"
)

var (
	configName = "main"
	configPath = "/config/"
	version    = "0.0.1"
	amqpHost   = ""
	amqpPort   = ""
	amqpUrl    = ""
	redisHost  = ""
	redisPort  = ""
	flags      = []cli.Flag{
		&cli.StringFlag{
			Name:        "config, c",
			Usage:       "path to .env config file",
			Destination: &configPath,
		},
	}
)

func parseEnvFile() {
	filepath, err := os.Getwd()
	if err != nil {
		panic("main, get rootDir error" + err.Error())
		return
	}
	viper.AddConfigPath(filepath + configPath)
	viper.SetConfigName(configName)
	err = viper.ReadInConfig()
	if err != nil {
		panic("main, fatal error while reading config file: " + err.Error())
		return
	}
	amqpHost = viper.GetString("rabbit.primary.host")
	amqpPort = viper.GetString("rabbit.primary.port")
	amqpUrl = viper.GetString("rabbit.primary.url")
	redisHost = viper.GetString("redis.primary.host")
	redisPort = viper.GetString("redis.primary.port")
	if amqpUrl == "" {
		amqpUrl = "amqps://" + amqpHost + ":" + amqpPort
	}
}

func run(c *cli.Context) error {
	parseEnvFile()
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	parseEnvFile()
	amqpConfig := amqp.Config{
		AMQPUrl: amqpUrl,
	}
	sess := amqp.NewSession(amqpConfig)
	err := sess.Connect()
	if err != nil {
		return err
	}
	clt, err := sess.Client(amqp.ClientConfig{})
	if err != nil {
		return err
	}
	redisStore, err := token_str.NewTokenStore(token_str.RedisConfig{
		Host: redisHost,
		Port: redisPort,
	})
	if err != nil {
		return err
	}
	amqpRequest := tik_api_lib.NewAmqpRequests(clt)
	service := tik_api_lib.NewService(amqpRequest, redisStore)
	httpEndpoints := tik_api_lib.NewHttpEndpoints(setdata_common.NewCommandHandler(service))
	mdw := token_str.NewMiddleware(redisStore)
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", httpEndpoints.MakeLoginEndpoint())
	}
	testGroup := r.Group("/test", mdw.MakeMiddleware())
	{
		testGroup.POST("/login", httpEndpoints.MakeLoginEndpoint())
	}
	_ = r.Run(port)
	return nil
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	app := cli.NewApp()
	app.Name = "tik external api"
	app.Description = ""
	app.Usage = "tik external run"
	app.UsageText = "tik external  run"
	app.Version = version
	app.Flags = flags
	app.Action = run

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
