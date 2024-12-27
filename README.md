# lotus-car
A simple CLI to generate car file and compute commp at the same time.

## Build
```sh
git clone https://github.com/minerdao/lotus-car.git
cd lotus-car
make
```
## Usage

```sh
./lotus-car -h
NAME:
   lotus-car - A tool for generating car files

USAGE:
   lotus-car [global options] command [command options] [arguments...]

COMMANDS:
   init         Initialize default configuration file
   init-db      Initialize database
   index        Index all files in target directory and save to json file
   generate     Generate car archive from list of files and compute commp
   regenerate   Regenerate car file from saved raw files information
   deal         Send deals for car files
   import-deal  Import proposed deals data to boost
   serve        Start API server
   user         Manage users
   help, h      Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --config value, -c value  Path to config file (default: "config.yaml")
   --help, -h                show help (default: false)
```

### Initialize default configuration file
```sh
./lotus-car init
```

### Initialize the database
```sh
./lotus-car init-db
```

Edit the config file, add postgres connection information, deal and API server configuration.

### Generate car files
```sh
./lotus-car generate --input=/ipfsdata/1712/1712.json --parent=/ipfsdata/1712/raw --tmp-dir=/ipfsdata/tmp1 --quantity=1 --out-dir=/ipfsdata/car --out-file=/home/fil/csv/dataset_1712_1227.csv
```
- **--input**：original file index file
- **--parent**：original file directory
- **--tmp-dir**：temporary directory
- **--quantity**：car file quantity
- **--out-dir**：car file output directory
- **--out-file**：output csv file name

### Regenerate car file from database
```sh
./lotus-car regenerate --id=86e7354d-d6ad-4fa3-b403-0790a567a3b4 --parent=/ipfsdata/dataset/1/raw --out-dir=/ipfsdata/car-regenerate
```
- **--id**：car file id in database
- **--parent**：original file directory
- **--out-dir**：car file output directory

### Make deals using boost
```sh
./lotus-car deal --miner=f0xxxxxx --from-wallet=f1fpvwsdrxxvd334s3jfeoinistcmbgxxyuqsxxxx --api="https://api.node.glif.io" --total=1
```
- **--miner**：miner address
- **--from-wallet**：client wallet address
- **--api**：boost api url
- **--total**：Number of deals to send in total (default: 1)

### Index source files
```sh
./lotus-car index --source-dir /ipfsdata/dataset/1/raw --output-dir /ipfsdata/dataset/1 --index-file 1.json
```
- **--source-dir**：source file directory
- **--output-dir**：output directory
- **--index-file**：output json file

The input file can be a text file that contains a list of file information SORTED by the path. i.e.
```json
[
  {
    "Path": "test/test.txt",
    "Size": 4038
  },
  {
    "Path": "test/test2.txt",
    "Size": 3089
  }
]
```

The tmp dir is useful when the dataset source is on slow storage such as NFS or S3FS/Goofys mount.

## API Server

### Start the API server
```sh
./lotus-car serve --port=8080
```
- **--port**：api server port
- **--config**：api server config file path


### Create admin user
```sh
./lotus-car user create --username=admin --password=admin_pwd
```
- **--username**：user name
- **--password**：user password


## Database migration
```sh
psql -d lotus_car -f db/migrations/rename_car_files_to_files.sql