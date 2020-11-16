Sometimes for network security , there are 3 requirements

1. Only allow dns query which query type is A.
2. Only allow query the domain name which in the whitelist .
3. When query a certain domain name , a special ip address should be return .

In order to achieve this goal , I wrote a go program , it read two files , one is the white list of the domain name , the other is the mapping between domain name and ip , after running , it will periodically read these two files to update the record .
