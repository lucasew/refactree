from collections import ChainMap, OrderedDict
import collections

class A:
    def run(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

def use_cm_od_sub():
    da = ChainMap(OrderedDict(k=A()))
    db = ChainMap(OrderedDict(k=B()))
    return da["k"].run() + db["k"].run()

def use_cm_od_var():
    da = ChainMap(OrderedDict(k=A()))
    db = ChainMap(OrderedDict(k=B()))
    xa = da["k"]
    xb = db["k"]
    return xa.run() + xb.run()

def use_cm_od_values():
    da = ChainMap(OrderedDict(k=A()))
    db = ChainMap(OrderedDict(k=B()))
    n = 0
    for a in da.values():
        n += a.run()
    for b in db.values():
        n += b.run()
    return n

def use_cm_od_nested_list():
    da = ChainMap(OrderedDict(k=[A()]))
    db = ChainMap(OrderedDict(k=[B()]))
    return da["k"][0].run() + db["k"][0].run()

def use_collections_cm_od():
    da = collections.ChainMap(collections.OrderedDict(k=A()))
    db = collections.ChainMap(collections.OrderedDict(k=B()))
    return da["k"].run() + db["k"].run()

def use_preserves_b():
    db = ChainMap(OrderedDict(k=B()))
    return db["k"].run()
