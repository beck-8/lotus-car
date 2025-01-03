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

### Send deals
```sh
# Run once with specific piece CIDs
./lotus-car deal --miner=f01234 --from-wallet=f1... --api="https://api.node.glif.io" --from-piece-cids=/path/to/piece_cids.txt --really-do-it --boost-client-path=/usr/local/bin/boost

piece_cids.txt:
baga6ea4seaqkue2e4krk6yn44e5m4ypemdhv25zrvva7v6ge2vond6agnhpe4ma
baga6ea4seaqaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
baga6ea4seaqbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
```

```sh
# Run once with pending files and total limit
./lotus-car deal --miner=f01234 --from-wallet=f1... --api="https://api.node.glif.io" --total=10 --really-do-it --boost-client-path=/usr/local/bin/boost

# Run every 1 hour (3600 seconds) with pending files and total limit
./lotus-car deal --miner=f01234 --from-wallet=f1... --api="https://api.node.glif.io" --total=10 --really-do-it --interval=3600 --boost-client-path=/usr/local/bin/boost
```
- **--miner**：Storage provider ID
- **--from-wallet**：Client wallet address
- **--api**：Lotus API endpoint (default: "https://api.node.glif.io")
- **--from-piece-cids**：Path to file containing piece CIDs (one per line). When specified, --total is ignored
- **--total**：Number of deals to send (default: 1). Ignored when --from-piece-cids is specified
- **--really-do-it**：Actually send the deals (default: false)
- **--interval**：Loop interval in seconds, 0 means run once (default: 0)
- **--boost-client-path**：Path to boost executable (overrides config file)
- **--start-epoch-day**：Start epoch in days (default: 10)
- **--duration**：Deal duration in epochs (default: 3513600, about 3.55 years)

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
    "Path": "dataset/1.tar",
    "Size": 4038
  },
  {
    "Path": "dataset/2.tar",
    "Size": 3089
  }
]

The tmp dir is useful when the dataset source is on slow storage such as NFS or S3FS/Goofys mount.
```

### Import deals
```sh
# Run once
./lotus-car import-deal --car-dir=/ipfsdata/car --boost-path=/usr/local/bin/boost --total=10

# Run every 300 seconds (5 minutes)
./lotus-car import-deal --car-dir=/ipfsdata/car --boost-path=/usr/local/bin/boost --interval=300 --total=10

./lotus-car import-deal --car-dir=/ipfsdata/car --boost-path=/usr/local/bin/boost --interval=300 --total=1 --regenerated=true
```
- **--car-dir**：car file directory
- **--boost-path**：path to boost executable
- **--interval**：loop interval in seconds (0 means run once)
- **--total**：number of deals to import
- **--regenerated**：only import deals with regenerated car files

### Export files
```sh
# Export all successful deals' piece CIDs
./lotus-car export-file --deal-status=success

# Export piece CIDs for deals in a specific time range
./lotus-car export-file --deal-status=success --start-time="2025-01-01 00:00:00" --end-time="2025-01-02 00:00:00"

# Export all deals' piece CIDs in a time range (regardless of status)
./lotus-car export-file --start-time="2025-01-01 00:00:00"
```
- **--deal-status**：Filter by deal status (pending/success/failed)
- **--start-time**：Filter by deal time start (format: YYYY-MM-DD HH:mm:ss)
- **--end-time**：Filter by deal time end (format: YYYY-MM-DD HH:mm:ss)

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

psql -d lotus_car -f db/migrations/add_updated_at.sql
psql -d lotus_car -f db/migrations/add_regenerate_status.sql

psql -d lotus_car -f db/migrations/rename_car_files_to_files.sql

```

## Release
```sh
git add .
git commit -m chore: prepare for v1.0.0 release
make release-common NEW_VERSION=v1.0.2
git push origin v1.0.2
./lotus-car version
```

