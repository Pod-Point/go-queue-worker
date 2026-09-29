# Formigo - distributed SQS worker pools.

> This library relies heavily on [`github.com/alitto/pond/v2`](https://github.com/alitto/pond), a very flexible and well-tested worker pool library.

Formigo is a fast, reliable worker pool consumer for SQS.

Basic features of Pond:

- automatic scaling to available resources/limits based on incoming queue pressure.
- fire & forget queue submission
- fire & **wait for a response** submission to a queue (e.g. for dependent jobs, or deferred behaviour).
- clean, graceful exit when shutting down.