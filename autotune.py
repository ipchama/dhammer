#!/usr/bin/python3

import argparse
import signal
import sys
import AutoTuner

"""
	Example script for interacting with the dhammer API

	Usage: autotune.py --tune-stat-name OfferReceived --tune-stat-compare-name DiscoverSent
"""

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
    
    parser.add_argument('--api-address','-a', dest='api_address', default='localhost',
                        help='Address for stats API.')

    parser.add_argument('--api-port','-p', dest='api_port', default=8080,
                        help='Port for stats API')
    
    parser.add_argument('--tune-stat-name','-t', dest='tune_stat_name', default=None, required=True,
                        help='Stat field used to decide if ramp up or down is necessary.')

    parser.add_argument('--tune-stat-compare-name','-c', dest='tune_stat_compare_name', default=None, required=True,
                        help='Stat used for comparison to determine if goal is being reached.')

    parser.add_argument('--tune-compare-min-percentage','-cp', dest='tune_compare_min_percentage', default=0.95,
                        help='The maximum percentage difference between the tuning stat and the comparison stat.')

    parser.add_argument('--ramp-up-factor','-ru', dest='ramp_up_factor', default=2, type=int,
                        help='Factor by which to ramp up the target RPS.')

    parser.add_argument('--ramp-down-factor','-rd', dest='ramp_down_factor', default=0.9, type=int,
                        help='Factor by which to reduce the ramp-up factor.')
    
    parser.add_argument('--refresh-rate_seconds','-rr', dest='refresh_rate_seconds', default=6, type=int,
                        help='Rate to check stats. Should be slightly longer than the dhammer refresh rate.')

    args = parser.parse_args()
    
    
    ##### Prep the result handler
    tuner = AutoTuner.AutoTuner(args)

    # Register our signal handler.
    signal.signal(signal.SIGINT, lambda signal, frame: signal_handler(signal, frame, tuner))

    try:
        tuner.prepare()
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
