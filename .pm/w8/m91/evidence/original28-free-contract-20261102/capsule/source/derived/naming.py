from datetime import datetime,timezone,timedelta
def utc_now(moment=None):
 moment=moment or datetime.now(timezone.utc)
 assert moment.tzinfo is not None
 return moment.astimezone(timezone.utc)

def creation_day(spec,moment=None):
 moment=utc_now(moment);day=moment.strftime('%Y%m%d')
 assert spec['name'].startswith('qa-'+day+'-') and spec['name_date_utc']==day,'stale prepared fixture name; refresh this unused spec'
 assert (moment.replace(hour=0,minute=0,second=0,microsecond=0)+timedelta(days=1)-moment).total_seconds()>120,'do not begin a create in the final120seconds of UTC day'
 return moment
