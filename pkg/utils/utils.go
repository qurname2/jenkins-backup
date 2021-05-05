package utils

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	log "github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"time"
)

//S3UploadObject upload object to aws S3 bucket
func S3UploadObject(bucketName string, filePath string) error {

	fileNameData, dstFileName :=  createCorrectFileName(filePath)

	file, err := os.Open(fileNameData)
	if err != nil {
		ExitErrorf("Unable to open file: ", err)
	}

	defer file.Close()

	// Initialize a session in Region from env() that the SDK will use to load
	// credentials from the shared credentials file ~/.aws/credentials.
	sess, errNewSession := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("S3_REGION")),
		MaxRetries: aws.Int(3)},
	)
	if errNewSession != nil {
		ExitErrorf(errNewSession)
	}

	// Setup the S3 Upload Manager.
	uploader := s3manager.NewUploader(sess)

	_, errUpload := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),

		Key: aws.String(dstFileName),
		// The file to be uploaded.
		Body: file,
	})
	if errUpload != nil {
		log.Errorf("Unable to upload %v to %v, %v", filePath, bucketName, errUpload)
		return errUpload
	}
	log.Infof("successfully uploaded %v to %v", dstFileName, bucketName)
	return nil
}

func ExitErrorf(args ...interface{}) {
	log.Info(args...)
	os.Exit(1)
}

//createCorrectFileName create name of archive with data and subfolder for S3 bucket.
//Example of result: /jenkins-backup/jenkins_home.tar.gz.04-30-2021
//You can find your archive in bucket like this: `aws s3 ls s3://<bucket-name>/jenkins-backup/`
func createCorrectFileName(filePath string) (string, string) {
	const layout = "01-02-2006"
	t := time.Now()
	fileNameData := filePath + "." + t.Format(layout)
	err := os.Rename(filePath, fileNameData)
	if err != nil {
		ExitErrorf("Unable to rename file: ", err)
	}
	dstFileNameBase := filepath.Base(fileNameData)
	dstFileName := filepath.Join("/jenkins-backup", dstFileNameBase)
	return fileNameData, dstFileName
}
