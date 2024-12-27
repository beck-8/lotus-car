# lotus-car
A simple CLI to generate car file and compute commp at the same time.

## Build
```sh
$ go build -o lotus-car
```
## Usage

#### Pack car file

```sh
$ ./lotus-car -h
NAME:
  generate - generate car archive from list of files and compute commp in the mean time

USAGE:
  generate [global options] command [command options] [arguments...]

COMMANDS:
  help, h  Shows a list of commands or help for one command

GLOBAL OPTIONS:
  --input value, -i value       File to read list of files, or '-' if from stdin (default: "-")
  --quantity value, -q value    Quantity of car files (default: 3)
  --file-size value             Target car file size, default to 32GiB size sector (default: 19327352832)
  --piece-size value, -s value  Target piece size, default to minimum possible value (default: 34359738368)
  --out-file value              Output file as .csv format to save the car file (default: "./source.csv")
  --out-dir value, -o value     Output directory to save the car file (default: ".")
  --tmp-dir value, -t value     Optionally copy the files to a temporary (and much faster) directory
  --parent value, -p value      Parent path of the dataset
  --help, -h                    show help (default: false)
```

./lotus-car generate --file-size=18874368 --piece-size=33554432 --input=/Users/max/data/dataset/1/1.json --parent=/Users/max/data/dataset/1/raw --tmp-dir=/Users/max/data/dataset/tmp --quantity=1 --out-dir=/Users/max/data/dataset/car --out-file=/Users/max/data/dataset/csv/1.csv

./lotus-car regenerate --id=86e7354d-d6ad-4fa3-b403-0790a567a3b4 --parent=/Users/max/data/dataset/1/raw --out-dir=/Users/max/data/dataset/car-regenerate

./lotus-car deal --use-boost --miner=f01824405 --from-wallet=f1fpvwsdrxxvd334s3jfeoinistcmbgxxyuqseywa --api="https://api.node.glif.io" --batch-size=1

./lotus-car index --source-dir /Users/max/data/dataset/1/raw --output-dir /Users/max/data/dataset/1 --index-file 1.json


The input file can be a text file that contains a list of file information SORTED by the path. i.e.
```json
[
  {
    "Path": "test/test.txt",
    "Size": 4038,
    "Start": 1000, # Inclusive
    "End": 2000 # Exclusive
  },
  {
    "Path": "test/test2.txt",
    "Size": 3089
  }
]
```

The output file is a .csv file that contains a list of `pieceCID,fileSize,pieceSize,dataCID`, i.e.
```csv
baga6ea4seaqm65rsjelthpzxl4xnki36yrio2xqphaxbi5v7jehltvgw7u2mgha,19683716501,34359738368,bafybeifvajapn6oa5wbmsxlxeffueb3ozzcuqxwcgoyradtqdukvzjaczu
baga6ea4seaqaw4j4spzjg7gkdh42gae6zoa42buyxlgvekhp3fpi2t4ym233idy,19202597332,34359738368,bafybeih7mkv4u2tdygwhnhpwfijir6pe62x653iwq2s2nlqj5r25m35hoe
baga6ea4seaqhdoz7ekvsunrlfdb5h4qhm3seu6kqgxnobfqn5apwfdmbwupeedq,19490669522,34359738368,bafybeig5ziizy3vdwqwyf37duf2vnjqmjflxuttphz563h24w7i3zmr54q
```

The tmp dir is useful when the dataset source is on slow storage such as NFS or S3FS/Goofys mount.
