# mineral-cli

## Configuration
1. Create a .env file
2. Copy this to your .env file
```bash
AWS_ENDPOINT=https://s3.poseidon.network
AWS_ACCESS_KEY_ID=<your access key>
AWS_SECRET_ACCESS_KEY=<your secret key>
AWS_REGION=us-east-1
AWS_DEFAULT_BUCKET=<your default bucket>
```

## Usage of mineral-cli

### Bucket

mineral-cli bucket ls
- This command retrieve the list of buckets 

mineral-cli bucket create --bucket=my-bucket
- This command retrieve the list of buckets 

### Object

mineral-cli ls
- Retrieve the list of objects

mineral-cli put --file=/home/user/my_text.txt --key=my_text.txt
- Upload file to mineral 

mineral-cli get --key=my_text.txt --filename=my_text.txt 
- Download file from mineral  

mineral-cli delete --key=my_text.txt
- Delete file from mineral