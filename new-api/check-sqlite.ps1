$ErrorActionPreference = 'Stop'

$dbPath = Join-Path $PSScriptRoot 'one-api.db'
if (-not (Test-Path $dbPath)) {
  throw "SQLite database not found: $dbPath"
}

@"
import sqlite3
p = r'$dbPath'
con = sqlite3.connect(p)
cur = con.cursor()
print('database:', p)
for table in ['users', 'channels', 'tokens', 'options', 'logs', 'redemptions', 'top_ups']:
    try:
        print(f'{table}:', cur.execute(f'select count(*) from {table}').fetchone()[0])
    except Exception as e:
        print(f'{table}: ERROR {e}')
try:
    rows = cur.execute('select id, username, display_name, role, status, quota, used_quota from users order by id limit 10').fetchall()
    print('users_sample:', rows)
except Exception as e:
    print('users_sample: ERROR', e)
try:
    rows = cur.execute('select id, name, type, status, base_url from channels order by id limit 10').fetchall()
    print('channels_sample:', rows)
except Exception as e:
    print('channels_sample: ERROR', e)
con.close()
"@ | python -
