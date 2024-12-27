# Postgresql数据库安装

参考文章：https://computingforgeeks.com/install-postgresql-14-on-ubuntu-jammy-jellyfish/

## 1. 安装postgresql数据库

```bash
sudo apt install vim curl wget gpg gnupg2 software-properties-common apt-transport-https lsb-release ca-certificates

sudo apt policy postgresql

curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc|sudo gpg --dearmor -o /etc/apt/trusted.gpg.d/postgresql.gpg

sudo sh -c 'echo "deb http://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'

sudo apt update -y


sudo apt install postgresql-14 -y

sudo systemctl status postgresql@14-main.service

sudo -u postgres psql -c "SELECT version();"

```

## 2. 配置远程连接

```bash
sudo vi /etc/postgresql/14/main/pg_hba.conf

`# IPv4 local connections:`下面增加：
host    all             all             182.18.83.0/24          trust
```
其中`182.18.83.0`为远程访问的白名单IP地址。

```bash
# Database administrative login by Unix domain socket
local   all             postgres                                peer

# TYPE  DATABASE        USER            ADDRESS                 METHOD

# "local" is for Unix domain socket connections only
local   all             all                                     peer
# IPv4 local connections:
host    all             all             127.0.0.1/32            scram-sha-256
host    all             all             182.18.83.0/24          trust
# IPv6 local connections:
host    all             all             ::1/128                 scram-sha-256
# Allow replication connections from localhost, by a user with the
# replication privilege.
local   replication     all                                     peer
host    replication     all             127.0.0.1/32            scram-sha-256
host    replication     all             ::1/128                 scram-sha-256
```

```bash
sudo vi /etc/postgresql/14/main/postgresql.conf
```

```bash
#------------------------------------------------------------------------------
# CONNECTIONS AND AUTHENTICATION
#-----------------------------------------------------------------------------
.......
listen_addresses='*'
```

```bash
sudo systemctl restart postgresql

sudo systemctl status postgresql
```

### 3. 初始化数据库

```bash
sudo -u postgres psql

postgres=# \du
```

```bash
CREATE DATABASE lotus_car;
ALTER USER postgres PASSWORD '123456';
GRANT ALL PRIVILEGES ON DATABASE lotus_car TO postgres;

postgres=# \q
```
