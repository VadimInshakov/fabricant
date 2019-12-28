## **Algotrading bot for EXMO cryptocurrency exchange**

#### HOWTO
Configure config.yaml:

    minprice: 400000.04 # minimum market price at which the bot makes transactions
    maxprice: 800000.0  # maximum market price at which the bot makes transactions
    gap: 1000.0            # the minimum difference between the purchase and sale price the bot reacts to
    useredis: true      # use redis for storing orders (true) or just a go map (false)
    dbaddr: localhost   # redis server address
    dbport: 6379        # redis server port
    dbpass:             # redis server pass
    dbnum: 0            # redis server db index

Set env variables with public and private EXMO API keys:

    export EXMO_PUBLIC=<YOUR PUB KEY>
    export EXMO_SECRET=<YOUR SECRET>

Then start Redis and Fabricant:

    docker run -p 6379:6379 --name redis -d redis redis-server --appendonly yes
    ./fabricant <args>
    
Args:

    --config - path to config file
    --withfund - boolean value, true means crypto already buyed, but sell order not created