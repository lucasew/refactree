from collections import OrderedDict
import collections


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_od_kw_sub():
    da = OrderedDict(k=A())
    db = OrderedDict(k=B())
    return da["k"].execute() + db["k"].run()


def use_od_kw_var():
    da = OrderedDict(k=A())
    db = OrderedDict(k=B())
    xa = da["k"]
    xb = db["k"]
    return xa.execute() + xb.run()


def use_od_kw_values():
    da = OrderedDict(k=A())
    db = OrderedDict(k=B())
    n = 0
    for a in da.values():
        n += a.execute()
    for b in db.values():
        n += b.run()
    return n


def use_od_kw_multi():
    da = OrderedDict(k=A(), m=A())
    db = OrderedDict(k=B(), m=B())
    return da["k"].execute() + da["m"].execute() + db["k"].run() + db["m"].run()


def use_od_pairs():
    da = OrderedDict([("k", A())])
    db = OrderedDict([("k", B())])
    return da["k"].execute() + db["k"].run()


def use_od_from_literal():
    da = OrderedDict({"k": A()})
    db = OrderedDict({"k": B()})
    return da["k"].execute() + db["k"].run()


def use_collections_od_kw():
    da = collections.OrderedDict(k=A())
    db = collections.OrderedDict(k=B())
    return da["k"].execute() + db["k"].run()


def use_od_fs_kw():
    da = OrderedDict(k=frozenset([A()]))
    db = OrderedDict(k=frozenset([B()]))
    return next(iter(da["k"])).execute() + next(iter(db["k"])).run()


def use_od_fs_kw_var():
    da = OrderedDict(k=frozenset([A()]))
    db = OrderedDict(k=frozenset([B()]))
    ga = da["k"]
    gb = db["k"]
    return next(iter(ga)).execute() + next(iter(gb)).run()


def use_od_fs_kw_for():
    da = OrderedDict(k=frozenset([A()]))
    db = OrderedDict(k=frozenset([B()]))
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_od_kw_match():
    da = OrderedDict(k=A())
    db = OrderedDict(k=B())
    match da:
        case {"k": xa}:
            r = xa.execute()
    match db:
        case {"k": xb}:
            r += xb.run()
    return r


def use_preserves_b():
    db = OrderedDict(k=B())
    return db["k"].run()
