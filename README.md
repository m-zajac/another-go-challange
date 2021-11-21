# Go coding challenge

An HTTP server that returns content from a list of providers based on the configuration.

Original requirements were redacted :)

## My take on the challenge

This task is simple to implement in a basic form but a bit tricky if we want to make calls to providers efficiently (using minimal requests per provider, fetching minimal amount of data, doing it all in parallel).

While implementing the code, my goals were:
- Guarantee:
  - at most 1 request per provider when handling a request, when there are no provider client errors,
  - at most 2 requests per provider when handling a request in the worst case (some provider failures)
- Call the providers concurrently to minimize response time,
- Ensure there is a timeout defined for processing incoming requests. Return status 500 when timeout is exceeded,
- Write readable code,
- Write solid tests.
- Keep everything as simple as possible while not bending good practices (e.g., single responsibility)

## Assumptions

- I didn't implement any form of caching.
- I didn't change the default configuration nor implement reading configuration from environment/files.
- I used the simplest logging with the standard `log` package and didn't differentiate between info and error logs.

I think these things are relevant, but I assumed they are out of the scope for this task. If needed, I can implement them later.

## Other notes

- I modified tests to use `httptest.NewServer`.
- I think using testify for testing would reduce the amount of test code. But due to the instructions, I didn't use it.
- The flag `addr` was not working correctly; I fixed that.
- I didn't use pre-existing `App` struct.
    - I wanted to separate the HTTP code from the app code, so I created a `Handler` and a `Service` types instead.
- I think adding a request identifier to the logs would be useful, but I assumed it is out of scope for this task.
- Example worst case scenario that leads to making two requests to one provider: 
  - config: [`providerA` (fallback: none), `providerB` (fallback: `providerA`)]
  - provider A returns ok, provider B fails. In this case 2 requests to provider A will be made.

## Running the code and making a request

Run the code:

    go run .

Make a request:

    http '127.0.0.1:8080/?count=3&offset=10'