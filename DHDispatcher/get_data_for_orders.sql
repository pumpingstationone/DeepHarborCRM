
CREATE OR REPLACE FUNCTION public.get_data_for_orders(p_orders text)
 RETURNS json
 LANGUAGE plpgsql
AS $function$
DECLARE
    orders json := p_orders;
	o json;

	/* person.id */
	dhID int := (orders ->> 'MemberID');
	dhIDConfirm int;
	topicName text;

	/* We always need to get the status of the member */
	membershipEnabled boolean;

	/* Building the orders */
	orderParts json[];
	part json;

	/* This is what we're going to be sending back to the caller */
	completeOrder json;
BEGIN
	/* This part just checks to see if the person is valid, otherwise bail */
	SELECT
		id INTO dhIDConfirm
	FROM
		person
	WHERE
		id = dhID;
	IF dhIDConfirm IS NULL THEN
		RETURN '{}';
	END IF;

	/* Is the member active? */
	SELECT
		(status ->> 'MembershipEnabled')::boolean INTO membershipEnabled
	FROM
		person
	WHERE
		id = dhID;

	/* And here's where we're going through all the orders and getting the appropriate json from the person table */
	FOR o IN
	SELECT
		*
	FROM
		json_array_elements(orders -> 'ChangeOrders')
		LOOP
			/* This gets us the appropriate json for the field */
			EXECUTE format('select %s from person where id = ' || dhID, (
					SELECT
						o ->> 'Order')) INTO part;

			/* We add whether membership is enabled at the data level because this is what
			 * will be sent to the remote system, which needs to know
			 */
			part := json_build_object('memberid', dhID, 'membershipenabled', membershipEnabled, 'data', part);
			
			/* And get the topic for this order */
			SELECT
				topic INTO topicName
			FROM
				orders
			WHERE
				name = o ->> 'Order';
			part := json_build_object('topic', topicName, 'send', part);
			orderParts := array_append(orderParts, part);
		END LOOP;

	/* Now put the whole thing together to return */
	completeOrder := json_build_object('orders', orderParts);	
	return completeOrder;
END;
$function$
;