# GrabOne

GrabOne is a graphical controller around yt-dlp: the user gives it a link, chooses what to save, and it runs the downloads.

## Language

### Downloads

**Job**:
One download request, created by one press of Download. A job covers a single video or a whole collection, and is the unit that is queued, run and cancelled.
_Avoid_: Media in the queue, item, task

**Slot**:
The capacity to run one job. The number of slots is the user's simultaneous downloads setting.

**Connections**:
How many transfers one running job may make at the same time to fetch its media. It is the user's setting for speeding up a single job, and is independent of the number of slots.
_Avoid_: Threads, parallel downloads, concurrency

**Speed settings**:
The user's choices that change how fast a job fetches its media without changing what is saved: connections, chunked transfer, and handing single-file streams to aria2c.

**Plain retry**:
A second attempt at a failed job with every speed setting dropped. A job gets at most one, and it stays the same job in the same slot.
_Avoid_: Fallback, re-queue

**Running job**:
A job that holds a slot. It is not part of the queue.

**Queue**:
The waiting jobs, in the order they will start. A job leaves the queue when it takes a slot or is cancelled.
_Avoid_: Download list, backlog

**Queue position**:
A waiting job's place in the queue, counted from 1. Position 1 starts next.

**Reorder**:
Changing a waiting job's queue position. It never interrupts a running job, and it does not reach inside a job to rearrange a collection's items.
