Simple Store
===

### Enable the store

To setup a simple store, a few configuration options need to be set.

To enable the simplestore, edit the configuration file to match:

```
[resources]
upstream = simplestore:/home/user/.brclient/store
```

To disable the store, just comment out the `upstream` line above.

Next, the payment type needs to be set.  The options are `ln`, `onchain`, or
it can be left empty for manual charging.  If using `onchain`, an optional
account may be set to receive funds:

```
[simplestore]
paytype=ln
;account=store
```

### Configuration

Once the store has been enabled in the configuration file, a store
template will be installed if the path does not exist. 
The directory will be created and filled with a sample, minimal store
into the path specified in the `upstream` line above.


#### Store Front
First, edit `index.tmpl` to introduce your store front.

#### Products
In the `products/` directory you will find example product template files.
They should be edited to fit your store.  These files can contain multiple
products or can be split into multiple files.  Deleting a file removes all
products within that file from your store.

An example product might be:

```
[[products]]
title = "My guitar solo"
sku = "1209391282"
description = """An MP3 file of my guitar solo"""
tags = ["music", "mp3", "guitar"]
price = 0.99
sendfilename = "guitar_solo.mp3"
```

In the above example, `guitar_solo.mp3` should be located in the defined
`upstream` directory.

### Viewing
To see your store within `brclient`, run the command `/pages local`.

