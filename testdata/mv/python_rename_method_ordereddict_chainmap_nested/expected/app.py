from collections import OrderedDict, ChainMap
import collections


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_od_kw_sub():
    da = OrderedDict(k=[A()])
    db = OrderedDict(k=[B()])
    return da["k"][0].execute() + db["k"][0].run()


def use_od_kw_for():
    da = OrderedDict(k=[A()])
    db = OrderedDict(k=[B()])
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_od_kw_var():
    da = OrderedDict(k=[A()])
    db = OrderedDict(k=[B()])
    ga = da["k"]
    gb = db["k"]
    return ga[0].execute() + gb[0].run()


def use_od_kw_match():
    da = OrderedDict(k=[A()])
    db = OrderedDict(k=[B()])
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_od_kw_multi():
    da = OrderedDict(k=[A()], m=[A()])
    db = OrderedDict(k=[B()], m=[B()])
    return da["k"][0].execute() + da["m"][0].execute() + db["k"][0].run() + db["m"][0].run()


def use_od_pairs_sub():
    da = OrderedDict([("k", [A()])])
    db = OrderedDict([("k", [B()])])
    return da["k"][0].execute() + db["k"][0].run()


def use_od_from_literal():
    da = OrderedDict({"k": [A()]})
    db = OrderedDict({"k": [B()]})
    return da["k"][0].execute() + db["k"][0].run()


def use_coll_od():
    da = collections.OrderedDict(k=[A()])
    db = collections.OrderedDict(k=[B()])
    return da["k"][0].execute() + db["k"][0].run()


def use_cm_sub():
    da = ChainMap({"k": [A()]})
    db = ChainMap({"k": [B()]})
    return da["k"][0].execute() + db["k"][0].run()


def use_cm_for():
    da = ChainMap({"k": [A()]})
    db = ChainMap({"k": [B()]})
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_cm_var():
    da = ChainMap({"k": [A()]})
    db = ChainMap({"k": [B()]})
    ga = da["k"]
    gb = db["k"]
    return ga[0].execute() + gb[0].run()


def use_cm_values():
    da = ChainMap({"k": [A()]})
    db = ChainMap({"k": [B()]})
    n = 0
    for ga in da.values():
        n += ga[0].execute()
    for gb in db.values():
        n += gb[0].run()
    return n


def use_cm_match():
    da = ChainMap({"k": [A()]})
    db = ChainMap({"k": [B()]})
    match da:
        case {"k": [xa, *_]}:
            r = xa.execute()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_cm_multi():
    da = ChainMap({"k": [A()]}, {"m": [A()]})
    db = ChainMap({"k": [B()]}, {"m": [B()]})
    return da["k"][0].execute() + da["m"][0].execute() + db["k"][0].run() + db["m"][0].run()


def use_coll_cm():
    da = collections.ChainMap({"k": [A()]})
    db = collections.ChainMap({"k": [B()]})
    return da["k"][0].execute() + db["k"][0].run()


def use_preserves_b():
    db = OrderedDict(k=[B()])
    return db["k"][0].run()
