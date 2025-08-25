## Transaction Service
This is a transaction service core with focus mainly on the transaction processing and account management. The aim was to create a robust transaction processor which can handle transactions with high concurrency and maintain data integrity.

### Code Structure
The codebase is organized into the following packages:

- `internal/handlers`: Contains the HTTP handlers for the API.
- `internal/repository`: Contains the repository for the database.
- `internal/service`: Contains the business logic for the transaction processing.
- `internal/database`: Contains the database connection and migrations.
- `internal/logger`: Contains the logger for the application.
- `internal/models`: Contains the models for the application.
- `internal/testutil`: Contains the test utilities for the application.

### Setup
To start the server its recommended to just simply use docker-compose 

```bash
docker-compose up --build
```

#### Important Curl Commands

GET Health Check:

```bash
curl http://localhost:8080/health
```

GET Account Balance:

```bash
curl --location --request GET 'http://localhost:8080/accounts/{account_id}'
```

POST Accounts:

```bash
curl --location --request POST 'http://localhost:8080/accounts' \
--header 'Content-Type: application/json' \
--data-raw '{
    "account_id": 123,
    "initial_balance": "100.23344"
}'
```

POST Transactions:

```bash
curl -X POST http://localhost:8080/transactions -H "Content-Type: application/json" -d '{
    "source_account_id": 123,
    "destination_account_id": 456,
    "amount": "100.12345"
}'
```

### Testing
tests use `testcontainers` to spin up a postgres database and run the tests against it.
<br>
To run the tests, use the following command:

```bash
go test ./...
```

### Analysis
#### Concurrency:
Via multiple tests, it was found that the system can handle high concurrency and maintain data integrity.

#### Isolation Level:
The system uses `READ COMMITTED` isolation level by default. This is a good choice for a transaction service as it provides good performance and data integrity.
<br>
I also tested with `REPEATABLE READ` and `SERIALIZABLE` isolation level and found that it can handle high concurrency and maintain data integrity but it is slower than `READ COMMITTED` with `ROW_LEVEL_LOCKING`. 
Due to low success rate of `SERIALIZABLE` and `REPEATABLE READ` I decided to not go with them since on a high concurrent situation it throws error and requires application to retry which slows down the throughput.
<br>
In our current usecase `READ COMMITED` with `ROW_LEVEL_LOCKING` fits perfectly without giving any issue though as the product would group this could create concurrency issue where there are more complicated db queries required. Hence going forword we can bumb up the isolation level but for now this works.
