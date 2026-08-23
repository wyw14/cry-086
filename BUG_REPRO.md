# Bug Reproduction

## Bug

Regulatory reports remain visible after their retention deadline.

## Trigger

Store one expired and one active report for a site, advance the service clock beyond the first report's retention end, and list the site reports.

## Observed error

The listing returns both reports instead of excluding the expired record.
