package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3Types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"

	"github.com/poseidon-network/mineral-cli/internal/utils"
)

const (
	PART_SIZE = 6_000_000
	RETRIES   = 2
)

var s3Client *s3.Client

type CustomReader struct {
	fp   *os.File
	size int64
	read int64
}

func (r *CustomReader) Read(p []byte) (int, error) {
	return r.fp.Read(p)
}

func (r *CustomReader) ReadAt(p []byte, off int64) (int, error) {
	n, err := r.fp.ReadAt(p, off)
	if err != nil {
		return n, err
	}

	// Got the length have read( or means has uploaded), and you can construct your message
	atomic.AddInt64(&r.read, int64(n))

	// I have no idea why the read length need to be div 2,
	// maybe the request read once when Sign and actually send call ReadAt again
	// It works for me
	// log.Printf("total read:%d    progress:%d%%\n", r.read/2, int(float64(r.read*100/2)/float64(r.size)))
	clearCurrentLine()
	fmt.Printf("total read:%d    progress:%d%%", r.read, int((float64(r.read)/float64(r.size))*100))

	return n, err
}

func (r *CustomReader) Seek(offset int64, whence int) (int64, error) {
	return r.fp.Seek(offset, whence)
}

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

func clearCurrentLine() {
	fmt.Print("\033[G\033[K")
}
func moveCursorUp() {
	fmt.Print("\033[A")
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

	var upload_loop bool = ctx.Bool("loop")

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
	if upload_loop {
		cnt := 0
		for {
			err = uploadLoopFiles(bucket_name, filesPath, cnt)
			if err != nil {
				log.Fatalf("Failed to upload files to %s/%s, %s\n", bucket_name, filesPath, err.Error())
			}
			cnt++
		}
	} else {
		err = uploadFiles(bucket_name, filesPath)
	}
	if err != nil {
		log.Fatalf("Failed to upload file from %s to bucket %s, %s\n", filesPath, bucket_name, err.Error())
		return
	}
	log.Printf("Successfully uploaded file to bucket %s\n", bucket_name)
}

func Upload(bucket_name string, key string, resp *s3.CreateMultipartUploadOutput, fileBytes []byte, partNum int) (completedPart s3Types.CompletedPart, err error) {
	var try int
	for try <= RETRIES {
		uploadResp, err := s3Client.UploadPart(context.TODO(), &s3.UploadPartInput{
			Body:          bytes.NewReader(fileBytes),
			Bucket:        aws.String(bucket_name),
			Key:           aws.String(key),
			PartNumber:    int32(partNum),
			UploadId:      resp.UploadId,
			ContentLength: int64(len(fileBytes)),
		})
		if err != nil {
			fmt.Println(err)
			if try == RETRIES {
				return s3Types.CompletedPart{}, err
			} else {
				try++
			}
		} else {
			return s3Types.CompletedPart{
				ETag:       uploadResp.ETag,
				PartNumber: int32(partNum),
			}, nil
		}
	}
	return s3Types.CompletedPart{}, nil
}

func _uploadFile(bucket_name string, key string, filename string) (err error) {
	// open file
	file, _ := os.Open(filename)
	defer file.Close()

	// Get file size
	stats, _ := file.Stat()
	fileSize := stats.Size()

	// put file in byteArray
	buffer := make([]byte, fileSize) // wouldn't want to do this for a large file because it would store a potentially super large file into memory
	file.Read(buffer)

	// start multipart upload
	createdResp, err := s3Client.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(key),
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Upload ID: %d", createdResp.UploadId)

	var start, currentSize int
	var remaining = int(fileSize)
	var partNum = 1
	var completedParts []s3Types.CompletedPart

	for start = 0; remaining != 0; start += PART_SIZE {
		if remaining < PART_SIZE {
			currentSize = remaining
		} else {
			currentSize = PART_SIZE
		}

		completed, err := Upload(bucket_name, key, createdResp, buffer[start:start+currentSize], partNum)
		if err != nil {
			_, err = s3Client.AbortMultipartUpload(context.TODO(), &s3.AbortMultipartUploadInput{
				Bucket:   createdResp.Bucket,
				Key:      createdResp.Key,
				UploadId: createdResp.UploadId,
			})
			if err != nil {
				log.Fatal(err)
			}
		}
		remaining -= currentSize
		fmt.Printf("Part %v complete, %v bytes remaining\n", partNum, remaining)
		completedParts = append(completedParts, completed)
		partNum++
	}

	// complete multipart upload
	input := &s3.CompleteMultipartUploadInput{
		Bucket:   createdResp.Bucket,
		Key:      createdResp.Key,
		UploadId: createdResp.UploadId,
		MultipartUpload: &s3Types.CompletedMultipartUpload{
			Parts: completedParts,
		},
	}
	_, err = s3Client.CompleteMultipartUpload(context.TODO(), input)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Completed upload")
	// log.Println(result)
	return nil
}

func uploadFile(bucket_name string, key string, filename string) error {
	var osFile *os.File
	var fileInfo os.FileInfo
	var reader *CustomReader
	var err error

	osFile, err = os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file %q, %v", filename, err)
	}
	defer osFile.Close()

	fileInfo, err = osFile.Stat()
	if err != nil {
		return err
	}

	reader = &CustomReader{
		fp:   osFile,
		size: fileInfo.Size(),
	}
	// Upload a new object "testobject" with the string "Hello World!" to our "newbucket".
	input := &s3.PutObjectInput{
		Bucket: aws.String(bucket_name),
		Key:    aws.String(key),
		Body:   reader,
	}

	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = 10 * 1024 * 1024 // The minimum/default allowed part size is 5MB
		u.Concurrency = 10            // default is 5
	})
	_, err = uploader.Upload(context.TODO(), input)
	fmt.Printf("\n")

	return err
}

func uploadFiles(bucket_name string, filesPath string) error {
	var uploadErr error = nil
	var filePath string = ""

	files, err := ioutil.ReadDir(filesPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath = path.Join(filesPath, file.Name())
		if file.IsDir() {
			uploadErr = uploadFiles(bucket_name, filePath)
		} else {
			uploadErr = uploadFile(bucket_name, filePath, filePath)
			if uploadErr == nil {
				log.Printf("Successfully uploaded file (%s) to bucket %s\n", filePath, bucket_name)
			} else {
				log.Printf("Failed to upload file (%s) to bucket %s, %s\n", filePath, bucket_name, uploadErr.Error())
				return uploadErr
			}
		}
	}

	return uploadErr
}

func uploadLoopFiles(bucket_name string, filesPath string, cnt int) error {
	var uploadErr error = nil
	var filePath string = ""
	var key string = ""

	files, err := ioutil.ReadDir(filesPath)
	if err != nil {
		return err
	}

	for _, file := range files {
		filePath = path.Join(filesPath, file.Name())
		if file.IsDir() {
			uploadErr = uploadLoopFiles(bucket_name, filePath, cnt)
		} else {
			key = filePath
			if cnt > 0 {
				key = key + "_" + strconv.Itoa(cnt)
			}
			uploadErr = uploadFile(bucket_name, key, filePath)
			if uploadErr == nil {
				log.Printf("Successfully uploaded file (%s) to bucket %s\n", key, bucket_name)
			} else {
				log.Printf("Failed to upload file (%s) to bucket %s, %s\n", key, bucket_name, uploadErr.Error())
				return uploadErr
			}
		}
	}

	return uploadErr
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
					&cli.BoolFlag{
						Name:  "loop",
						Usage: "Loop through files in the directory",
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
