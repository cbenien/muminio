#!/usr/bin/env/python
import os
import time
from datetime import datetime
import boto3
from botocore.client import Config


minio_url = os.environ["MINIO_URL"]
minio_secure = os.environ["MINIO_SECURE"]
minio_access_key = os.environ["MINIO_ACCESS_KEY"]
minio_secret_key = os.environ["MINIO_SECRET_KEY"]
minio_bucket_name = os.environ["MINIO_BUCKET_NAME"]

if minio_secure.lower() == "true":
    minio_url = "https://" + minio_url
else:
    minio_url = "http://" + minio_url

s3 = boto3.resource('s3',
                    endpoint_url=minio_url,
                    aws_access_key_id=minio_access_key,
                    aws_secret_access_key=minio_secret_key,
                    config=Config(signature_version='s3v4'),
                    region_name='us-east-1')

while True:
    for i in range(10):
        data = datetime.now().strftime("%c").encode('utf-8')

        object_name = f'object-{i}.txt'
        obj = s3.Object(minio_bucket_name, object_name)

        print(f'Writing object {minio_bucket_name}/{object_name}')
        obj.put(Body=data)

        print(f'Reading object {minio_bucket_name}/{object_name}')
        downloaded = obj.get()['Body']
        body = downloaded.read()
        if body == data:
            print("Downloaded data matches!")
        else:
            print("Downloaded data doesn't match")
            os.exit(1)

    time.sleep(5)
