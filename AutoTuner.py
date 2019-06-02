#!/usr/bin/python3

import requests
import json
import logging
import os
import time
import math

class AutoTuner:
    """A class for using the dhammer API to find the optimal rps rate."""

    def __init__(self, options):
        self._options = options
        self._options.tune_compare_min_percentage = self._options.tune_compare_min_percentage / 100        
        self._run = True
        self._rps = 1

        response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/1")
        response.raise_for_status()
        time.sleep(self._options.refresh_rate_seconds)

    def _tune(self):
        logging.debug("Going to tune...")

        response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/stats")
        response.raise_for_status()
        stats = json.loads(response.text)
        
        target_stat = None
        compare_stat = None
        
        for stat in stats:
            if stat['stat_name'] == self._options.tune_stat_name:
                target_stat = stat
            elif stat['stat_name'] == self._options.tune_stat_compare_name:
                compare_stat = stat
        
        diff_perc = target_stat['stat_rate_per_second'] / compare_stat['stat_rate_per_second']
                
        if  diff_perc >= self._options.tune_compare_min_percentage:
            self._rps = math.floor(self._rps * self._options.ramp_up_rate)
            response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/{self._rps}")
            response.raise_for_status()
            print(f"{target_stat['stat_name']} {target_stat['stat_rate_per_second']} / {compare_stat['stat_name']} {compare_stat['stat_rate_per_second']} = {diff_perc}: ramped up. New target RPS is {self._rps}")

        else:
            self._rps = math.floor(self._rps * self._options.ramp_down_rate)
            response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/{self._rps}")
            response.raise_for_status()
            print(f"{target_stat['stat_name']} {target_stat['stat_rate_per_second']} < {compare_stat['stat_name']} {compare_stat['stat_rate_per_second']} = {diff_perc}: ramped down. New target RPS is {self._rps}")

        return(True)
  
    def stop(self):
        self._run = False
        return(True)


    def start(self):      
        while self._run and self._tune(): # GIL for now ;D
            time.sleep(self._options.refresh_rate_seconds)
            

