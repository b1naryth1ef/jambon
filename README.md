# jambon - tacview / acmi file processing utility

jambon is a small utility designed to help process large tacview (ACMI). It includes CLI tools for searching objects within a tacview, determining object life span, trimming tacviews to specific time frames, and filtering out objects.

### Performance

jambon allows the user to optimize for speed or reduced memory usage when running ACMI processing commands. Commands that read ACMI files have a `--concurrency` flag which determines the number of data-processing routines that will be started. Generally speaking if a command outputs an ACMI file, a `--concurrency` of 1 will provide a consistent and small memory usage pattern. A larger concurrency value obviously results in less time processing but will require much more memory as data is buffered between stages.

## Searching

We can search by any object property and jambon will produce time frames for all relevant objects.

```bash
$ jambon search --property "Pilot=Tracer 1-1 | Apothecary" --file example.acmi
Processing file test.acmi...
Object 2051
  First Seen: 2021-07-24T04:00:47Z (47.18)
  Last Seen:  2021-07-24T04:23:00Z (1380.33)
Object 171523
  First Seen: 2021-07-24T04:23:07Z (1387.45)
  Last Seen:  2021-07-24T04:36:22Z (2182.34)
Object 348163
  First Seen: 2021-07-24T04:36:54Z (2214.48)
  Last Seen:  2021-07-24T04:49:39Z (2979.97)
Object 551171
  First Seen: 2021-07-24T04:51:16Z (3076.77)
  Last Seen:  2021-07-24T04:54:36Z (3276.82)
Object 578051
  First Seen: 2021-07-24T04:54:39Z (3279.72)
  Last Seen:  2021-07-24T05:32:01Z (5521.57)
```

Or perhaps you prefer structured data:

```bash
$ jambon search --property "Pilot=Tracer 1-1 | Apothecary" --file example.acmi --json | jq '.'
[
  {
    "object": {
      "Id": 2051,
      "Properties": [
        {
          "Key": "T",
          "Value": "3.3380541|6.0067414|44.88||4.7|95.6|242348.11|-5254.55|92.5"
        },
        {
          "Key": "Type",
          "Value": "Air+FixedWing"
        },
        {
          "Key": "Name",
          "Value": "AV8BNA"
        },
        {
          "Key": "Pilot",
          "Value": "Tracer 1-1 | Apothecary"
        },
        {
          "Key": "Group",
          "Value": "Ford 3"
        },
        {
          "Key": "Color",
          "Value": "Blue"
        },
        {
          "Key": "Coalition",
          "Value": "Enemies"
        },
        {
          "Key": "Country",
          "Value": "us"
        }
      ],
      "Deleted": false
    },
    "first_seen": 47.18,
    "last_seen": 1380.33
  },
...
]
```

## Trimming

Once we have a time frame we can utilize the trim functionality to produce a much smaller ACMI file.

```
$ jambon trim --input before.acmi --start-at-offset-time 3279.72 --end-at-offset-time 5521.57 --output after.acmi
Collecting frames between 3279.72 and 5521.57...
Sorting 47240 collected frames...
Writing 47240 frames...
```