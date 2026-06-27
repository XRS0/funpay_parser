import os, subprocess, sys
# Start dev.py detached and redirect output to dev.log
with open('dev.log', 'a') as log:
    proc = subprocess.Popen(
        [sys.executable, 'dev.py'],
        stdout=log,
        stderr=subprocess.STDOUT,
        start_new_session=True,
        close_fds=True,
        cwd=os.getcwd(),
    )
    print('started', proc.pid)
