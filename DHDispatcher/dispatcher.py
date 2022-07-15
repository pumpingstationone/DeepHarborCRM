#!/usr/bin/env python3

import time
import sys
import psycopg2
import configparser
from json import dumps
from json import loads
import stomp

###############################################################################
# Globals
###############################################################################

config = configparser.ConfigParser()
config.read('settings.ini')

# Our database info which we read from the ini file
conn = psycopg2.connect(
    host=config['Database']['host'],
    database=config['Database']['database'],
    user=config['Database']['user'],
    password=config['Database']['password'])

class DHQListener(stomp.ConnectionListener):
	def on_error(self, headers, message):
		print('received an error "%s"' % message)

	def on_message(self, message):
		print('received a message "%s"' % message.body)
		# Now let's get the data we need for these orders
		orderData = getDataForOrders(message.body)    
		# And hand it off for them to be processed...
		processOrders(orderData)

        
# Now let's connect to the Apache MQ server
hosts = [(config['QueueServer']['host'], config['QueueServer']['port'])]
qconn = stomp.Connection(host_and_ports=hosts)
qconn.set_listener('', DHQListener())
qconn.connect(config['QueueServer']['user'], config['QueueServer']['password'], wait=True)
# And now register the consumer
qconn.subscribe(destination=config['QueueServer']['queue'], id=config['QueueServer']['qid'], ack='auto')
                            
###############################################################################
# Functions that actually do stuff
###############################################################################

# This function makes a call to the plpgsql function "get_data_for_orders" which
# does a lot of the heavy lifting of getting the data associated with the
# individual orders. It returns all the data related to the orders for the specific
# member, where it gets cut up and sent to the appropriate topics for the
# downstream programs to handle it
def getDataForOrders(orders):
	dataCursor = conn.cursor()
	dataCursor.execute(f"select get_data_for_orders('{orders}')")
	
	rawData = ''
	for data in dataCursor:
		rawData = data[0]
	dataCursor.close()
	
	return rawData

# Here we have received the data associated with the orders, so we are going
# to go through the array and send the data to the appropriate topics, which
# we'll get from the data itself
def processOrders(orderData):
	def sendToTopic(topicName, dataToSend):
		qconn.send(body=dumps(dataToSend), destination=topicName)
	
	#print(f"----> {orderData}")
	if len(orderData) == 0:
		print("Hmm, no data?")
		return
		
	for o in orderData['orders']:
		sendToTopic(o['topic'], o['send'])
	
###############################################################################
# Start of program
###############################################################################

print("*** DH Dispatcher Starting ***")

while True:
	time.sleep(2)
	print("Waiting for orders...")
	
qconn.disconnect()

# Should never get here
print("*** DH Dispatcher Exited ***")