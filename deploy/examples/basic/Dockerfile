FROM python:3

WORKDIR /usr/src/app

COPY requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

ENV MINIO_URL=10.96.82.4:9000
ENV MINIO_SECURE=false
ENV MINIO_ACCESS_KEY=MINIO_ACCESS_KEY_12345
ENV MINIO_SECRET_KEY=74effa8a8e9b7474de606a818183c188d2bf158b8790d6783d02a100f58c82d2
ENV MINIO_BUCKET_NAME=bucket-basic-example

CMD [ "python", "./basic.py" ]
