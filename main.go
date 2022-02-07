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
	configName              = "main"
	configPath              = "/config/"
	version                 = "0.0.1"
	amqpHost                = ""
	amqpPort                = ""
	amqpUrl                 = ""
	redisHost               = ""
	redisPort               = ""
	s3endpoint              = "https://s3.us-east-2.amazonaws.com"
	s3bucket                = "setdata"
	s3accessKey             = "AKIA54CP6OJQEHUI6KFO"
	s3secretKey             = "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"
	s3uploadedFilesBasePath = "https://setdata.s3.us-east-2.amazonaws.com"
	s3region                = "us-east-2"
	port                    = "8080"
	flags                   = []cli.Flag{
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

	s3, err := tik_api_lib.NewS3Uploader(
		s3endpoint,
		s3accessKey,
		s3secretKey,
		s3bucket,
		s3uploadedFilesBasePath,
		s3region)
	if err != nil {
		return err
	}

	service := tik_api_lib.NewService(amqpRequest, redisStore, s3)
	httpEndpoints := tik_api_lib.NewHttpEndpoints(setdata_common.NewCommandHandler(service))
	mdw := token_str.NewMiddleware(redisStore)
	gin.SetMode(gin.ReleaseMode)

	r := gin.Default()
	authGroup := r.Group("/auth")
	{
		authGroup.POST("/login", httpEndpoints.MakeLoginEndpoint())
		authGroup.POST("/register", httpEndpoints.MakeRegisterEndpoint())
	}
	filesGroup := r.Group("/files", mdw.MakeMiddleware())
	{
		filesGroup.POST("/", httpEndpoints.MakeCreateFileEndpoint())
		filesGroup.GET("/", httpEndpoints.MakeListFilesEndpoint())
		filesGroup.GET("/id", httpEndpoints.MakeGetFileByIdEndpoint())
		filesGroup.PUT("/", httpEndpoints.MakeUpdateFileEndpoint())
		filesGroup.PUT("/file", httpEndpoints.MakeUploadFileEndpoint())
		filesGroup.DELETE("/", httpEndpoints.MakeDeleteFileEndpoint())
	}
	return r.Run()
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
