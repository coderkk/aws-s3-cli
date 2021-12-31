package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/poseidon-network/mineral-cli/internal/utils"
)

var s3Client *s3.Client

func EstablishConnection() {
	var cfg aws.Config
	var err error
	_ = godotenv.Load()

	creds := credentials.NewStaticCredentialsProvider(os.Getenv("AWS_ACCESS_KEY_ID"), os.Getenv("AWS_SECRET_ACCESS_KEY"), "")
	customResolver := aws.EndpointResolverFunc(func(service, region string) (aws.Endpoint, error) {
		// if awsEndpoint != "" {
		return aws.Endpoint{
			PartitionID:   "aws",
			URL:           os.Getenv("AWS_ENDPOINT"),
			SigningRegion: os.Getenv("AWS_REGION"),
		}, nil
		//}

		// returning EndpointNotFoundError will allow the service to fallback to it's default resolution
		// return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	// Using the SDK's default configuration, loading additional config
	// and credentials values from the environment variables, shared
	// credentials, and shared configuration files
	cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(creds), config.WithEndpointResolver(customResolver))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	// Create an Amazon S3 service client
	s3Client = s3.NewFromConfig(cfg)
}

func GetBucketList(ctx *cli.Context) {
	// Get the first page of results for ListBuckets for a bucket
	result, err := s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		log.Fatal(err)
	}

	totalBuckets := 0
	log.Println("List of buckets:")
	for _, bucket := range result.Buckets {
		fmt.Println("- " + *bucket.Name + ": " + bucket.CreationDate.Format("2006-01-02 15:04:05 Monday"))
	}
	totalBuckets = len(result.Buckets)
	fmt.Println("Total buckets:", totalBuckets)
}

func CreateBucket(ctx *cli.Context) {
	var bucket_name string = ctx.String("bucket")

	// Get the first page of results for ListBuckets for a bucket
	_, err := s3Client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket: aws.String(bucket_name),
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Successfully created bucket")
}

func GetObjectList(ctx *cli.Context) {
	var bucket_name string = ctx.String("bucket")
	if bucket_name == "" {
		bucket_name = os.Getenv("AWS_DEFAULT_BUCKET")
	}

	log.Println("List of objects (" + bucket_name + "):")
	totalObjects := 0
	// Get the first page of results for ListObjectsV2 for a bucket
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket_name),
	})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(context.TODO())
		if err != nil {
			// handle error
			log.Fatal(err)
		}
		for _, object := range output.Contents {
			fmt.Printf("- key=%s size=%d\n", aws.ToString(object.Key), object.Size)
		}
		totalObjects += len(output.Contents)
	}
	fmt.Println("Total objects:", totalObjects)

	// // Get the first page of results for ListObjectsV2 for a bucket
	// output, err := s3Client.ListObjectsV2(context.TODO(), &s3.ListObjectsV2Input{
	// 	Bucket: aws.String(bucket_name),
	// })
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// log.Println("List of objects (" + bucket_name + "):")
	// for _, object := range output.Contents {
	// 	fmt.Printf("- key=%s size=%d\n", aws.ToString(object.Key), object.Size)
	// }
}

func PutObject(ctx *cli.Context) {
	var currentPath string
	var err error
	var bucket_name string = ctx.String("bucket")
	if bucket_name == "" {
		bucket_name = os.Getenv("AWS_DEFAULT_BUCKET")
	}

	var key string = ctx.String("key")
	if key == "" {
		key = uuid.NewString()
	}

	var file string = ctx.String("file")
	if !utils.IsFileExists(file) {
		currentPath, err = utils.GetExecutePath()
		if err == nil {
			var newUploadFile string = strings.Replace(file, "./", currentPath, -1)
			if utils.IsFileExists(newUploadFile) {
				file = newUploadFile
			} else {
				newUploadFile = currentPath + file
				if utils.IsFileExists(newUploadFile) {
					file = newUploadFile
				} else {
					log.Fatal("File doesn't exist")
				}
			}
		} else {
			log.Fatal("File doesn't exist")
		}
	}

	err = uploadFile(bucket_name, key, file)
	if err != nil {
		fmt.Println("Got error uploading file:")
		fmt.Println(err)
		return
	}
	if err != nil {
		log.Fatalf("Failed to upload file to %s/%s, %s\n", bucket_name, key, err.Error())
		return
	}
	log.Printf("Successfully uploaded file with key %s to bucket %s\n", key, bucket_name)
}

func PutObjects(ctx *cli.Context) {
	var currentPath string
	var err error
	var bucket_name string = ctx.String("bucket")
	if bucket_name == "" {
		bucket_name = os.Getenv("AWS_DEFAULT_BUCKET")
	}

	var filesPath string = ctx.String("path")
	if !utils.IsDirExists(filesPath) {
		currentPath, err = utils.GetExecutePath()
		if err == nil {
			var newUploadPath string = strings.Replace(filesPath, "./", currentPath, -1)
			if utils.IsDirExists(newUploadPath) {
				filesPath = newUploadPath
			} else {
				newUploadPath = currentPath + filesPath
				if utils.IsDirExists(newUploadPath) {
					filesPath = newUploadPath
				} else {
					log.Fatal("Path does not exist")
				}
			}
		} else {
			log.Fatal("Path does not exist")
		}
	}

	log.Printf("Start upload to %s\n", filesPath)
	err = uploadFiles(bucket_name, "", filesPath)
	if err != nil {
		log.Fatalf("Failed to upload file from %s to bucket %s, %s\n", filesPath, bucket_name, err.Error())
		return
	}
	log.Printf("Successfully uploaded file to bucket %s\n", bucket_name)
}

func uploadFile(bucket_name string, key string, file string) error {
	buff, err := utils.CreateReadStream(file)
	if err != nil {
		fmt.Println("Failed to open a file.", err)
		os.Exit(1)
	}

	// Upload a new object "testobject" with the string "Hello World!" to our "newbucket".
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(key),
		Body:   bytes.NewReader(buff),
	}

	uploader := manager.NewUploader(s3Client)
	_, err = uploader.Upload(context.TODO(), input)

	return err
}

func uploadFiles(bucket_name string, key string, filesPath string) error {
	var err error
	var uploadErr error = nil
	err = filepath.Walk(filesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		log.Printf("File %s\n", path)
		if info.IsDir() {
			if path != filesPath {
				uploadErr = uploadFiles(bucket_name, "", path)
			}
		} else {
			uploadErr = uploadFile(bucket_name, path, path)
			if uploadErr == nil {
				log.Printf("Successfully uploaded file (%s) to bucket %s\n", path, bucket_name)
			}
		}
		return nil
	})
	return err
}

func GetObject(ctx *cli.Context) {
	var err error
	var bucket_name string = ctx.String("bucket")
	if bucket_name == "" {
		bucket_name = os.Getenv("AWS_DEFAULT_BUCKET")
	}

	var key string = ctx.String("key")

	var filename string = ctx.String("filename")
	if filename == "" {
		filename = key
	}

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(key),
	}

	// Create the file
	newFile, err := os.Create(filename)
	if err != nil {
		log.Println(err)
	}
	defer newFile.Close()

	downloader := manager.NewDownloader(s3Client)
	numBytes, err := downloader.Download(context.TODO(), newFile, input)

	if err != nil {
		log.Println("Got an error download item:")
		log.Println(err)
		return
	}

	log.Printf("Downloaded %s (%d) from %s", key, numBytes, bucket_name)
}

func DeleteObject(ctx *cli.Context) {
	var err error
	var bucket_name string = ctx.String("bucket")
	if bucket_name == "" {
		bucket_name = os.Getenv("AWS_DEFAULT_BUCKET")
	}

	var key string = ctx.String("key")

	input := &s3.DeleteObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(key),
	}

	_, err = s3Client.DeleteObject(context.TODO(), input)
	if err != nil {
		log.Println("Got an error deleting item:")
		log.Println(err)
		return
	}

	log.Println("Deleted " + key + " from " + bucket_name)
}

func main() {
	var err error

	EstablishConnection()
	app := &cli.App{
		Commands: []*cli.Command{
			{
				Name:     "bucket",
				Category: "Bucket",
				Usage:    "Bucket",
				Subcommands: []*cli.Command{
					{
						Name:  "ls",
						Usage: "Retrieves a list of buckets",
						Action: func(c *cli.Context) error {
							GetBucketList(c)
							return nil
						},
					},
					{
						Name:  "create",
						Usage: "Create bucket",
						Flags: []cli.Flag{
							&cli.StringFlag{
								Name:  "bucket",
								Usage: "Name of the bucket",
							},
						},
						Action: func(c *cli.Context) error {
							CreateBucket(c)
							return nil
						},
					},
				},
			},
			{
				Name:     "ls",
				Category: "Object",
				Usage:    "Retrieves a list of objects",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "bucket",
						Usage: "Name of the bucket",
					},
				},
				Action: func(c *cli.Context) error {
					GetObjectList(c)
					return nil
				},
			},
			{
				Name:     "put",
				Category: "Object",
				Usage:    "Put object",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "file",
						Required: true,
						Usage:    "File location",
					},
					&cli.StringFlag{
						Name:  "key",
						Usage: "Key name of the object to put",
					},
					&cli.StringFlag{
						Name:  "bucket",
						Usage: "name of the bucket",
					},
				},
				Action: func(c *cli.Context) error {
					PutObject(c)
					return nil
				},
			},
			{
				Name:     "puts",
				Category: "Object",
				Usage:    "Put objects",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "path",
						Required: true,
						Usage:    "File path location",
					},
					&cli.StringFlag{
						Name:  "bucket",
						Usage: "name of the bucket",
					},
				},
				Action: func(c *cli.Context) error {
					PutObjects(c)
					return nil
				},
			},
			{
				Name:     "get",
				Category: "Object",
				Usage:    "Get object",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "key",
						Required: true,
						Usage:    "Key name of the object to get",
					},
					&cli.StringFlag{
						Name:  "filename",
						Usage: "Save to filename",
					},
					&cli.StringFlag{
						Name:  "bucket",
						Usage: "Name of the bucket",
					},
				},
				Action: func(c *cli.Context) error {
					GetObject(c)
					return nil
				},
			},
			{
				Name:     "delete",
				Category: "Object",
				Usage:    "Delete object",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "key",
						Required: true,
						Usage:    "Key name of the object to delete",
					},
					&cli.StringFlag{
						Name:  "bucket",
						Usage: "Name of the bucket",
					},
				},
				Action: func(c *cli.Context) error {
					DeleteObject(c)
					return nil
				},
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
