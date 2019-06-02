#!/usr/bin/python3

import argparse
import signal
import sys
import AutoTuner

# autotune.py --api-address localhost --api-port 8080 --tune-field stat_rate_per_second --ramp-up-rate 2 --ramp-down_rate 0.3 --stop-when-tuned true

##### Install a signal handler for CTRL+C #####

def signal_handler(signal, frame, tuner):
    try:
        # Try to shut things down gracefully.
        print('Stopping...')
        tuner.stop()
    except BaseException as e:
        print(str(e))
        pass
    
    print('Shutting down...')
    sys.exit(0) # Raise a SystemExit exception

def main():

    ##### Get command-line arguments
    parser = argparse.ArgumentParser(description='Find the max request rate.')
    
    parser.add_argument('--api-address','-a', dest='api_address', default='localhost', required=False,
                        help='Address for stats API.')

    parser.add_argument('--api-port','-p', dest='api_port', default=8080,
                        help='Port for stats API')
    
    parser.add_argument('--tune-stat-name','-t', dest='tune_stat_name', default=None, required=True,
                        help='Stats field used to decide if ramp up or down is necessary.')

    parser.add_argument('--tune-stat-compare-name','-c', dest='tune_stat_compare_name', default=None, required=True,
                        help='Stats to compare if the desired goal is to have one rate match another.')

    parser.add_argument('--tune-compare-min-percentage','-cp', dest='tune_compare_min_percentage', default=95, required=False,
                        help='If comparing stats, the minimum percentage match required to consider the tune goal reached.')

    parser.add_argument('--ramp-up-rate','-ru', dest='ramp_up_rate', default=2, type=int,
                        help='Rate to ramp up.')

    parser.add_argument('--ramp-down-rate','-rd', dest='ramp_down_rate', default=0.7, type=int,
                        help='Rate to ramp up.')
    
    parser.add_argument('--refresh-rate_seconds','-rr', dest='refresh_rate_seconds', default=5, type=int,
                        help='Rate to check.')

    parser.add_argument('--stop-when-tuned','-s', dest='stop_when_tuned', default=False, type=bool,
                        help='Stop when it seem the optimal rate has been reached.')
    

    args = parser.parse_args()
    
    
    ##### Prep the result handler
    tuner = AutoTuner.AutoTuner(args)

    # Register our signal handler.
    signal.signal(signal.SIGINT, lambda signal, frame: signal_handler(signal, frame, tuner))

    try:
        tuner.start()
    except SystemExit:
        pass
    except BaseException as e:
        print("Tuner broke down: %s" % str(e))

    try:
        tuner.stop()
    except:
        pass
    
    return(0)

if __name__ == "__main__":
    main()
