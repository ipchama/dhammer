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
        self._run = True
        self._rps = 1
        
        self._previous_ramped_up_rps = 1

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
        
        info = {}
                
        if  diff_perc >= self._options.tune_compare_min_percentage:
            self._previous_ramped_up_rps = self._rps
            self._rps = self._rps * self._options.ramp_up_factor             

            response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/{math.floor(self._rps)}")
            response.raise_for_status()
            
            print(f"{target_stat['stat_name']} / {compare_stat['stat_name']} = {diff_perc}: ramped up. Target RPS is {self._rps}.")

        else:
            
            self._options.ramp_up_factor = self._options.ramp_up_factor * self._options.ramp_down_factor

            if self._options.ramp_up_factor <= 1:
                 print(f"{target_stat['stat_name']} / {compare_stat['stat_name']} = {diff_perc}: Ramp down triggered, but next ramp-up difference too low.")
                 print(f"Target reached. Optimal RPS is approximately {math.floor(self._previous_ramped_up_rps)}.")
                 return(False)
             
            self._rps = self._previous_ramped_up_rps * self._options.ramp_up_factor
            
            response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/{math.floor(self._rps)}")
            response.raise_for_status()
            print(f"{target_stat['stat_name']} / {compare_stat['stat_name']} = {diff_perc}: ramped down. Target RPS is {self._rps}.")

        return(True)

    def prepare(self):
        response = requests.get(f"http://{self._options.api_address}:{self._options.api_port}/update/rps/1")
        response.raise_for_status()
        time.sleep(self._options.refresh_rate_seconds)
  
  
    def stop(self): # GIL for now ;D
        self._run = False
        return(True)


    def start(self):      
        while self._run and self._tune(): # GIL for now ;D
            time.sleep(self._options.refresh_rate_seconds)
            

