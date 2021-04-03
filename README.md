# The Service
As stated in the assignment, the service accepts POST request to `/callback` route with an array of object IDs in the request body.

On each callback request it starts the asynchronous update of those objects and responds to the client immediately with 202 Accepted.

For each object the service requests its data by ID from the `tester_service` and updates object's state in the database.

## How to run

`go run service.go`

## Object updates
Theoretically, if this were a production system, there could be a race condition.

Assume we have 2 concurrent updates to the same object (`updateTime` values are for the sake of example):
```
Update 1 = {
    id: 1,
    online: true,
    updateTime: 100
}

Update 2 = {
    id: 1,
    online: false,
    updateTime: 200
}
```

There is a chance that `Update 1` applies after `Update 2` even if it is not the most recent (basing on `updateTime`).
This can leave an object in an invalid state.

This service uses `last_updated` object field as a version marker and applies only the newest updates.
More detailed description is in `db.go` for method `UpdateLastSeen`.

## Stale objects cleanup
Every 30 seconds, all objects which were updated more than 30 seconds ago are removed from the database.
