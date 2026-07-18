from collections import defaultdict
import collections


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_append_sub():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].append(A())
    db["k"].append(B())
    return da["k"][0].execute() + db["k"][0].run()


def use_append_var():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].append(A())
    db["k"].append(B())
    ga = da["k"]
    gb = db["k"]
    return ga[0].execute() + gb[0].run()


def use_append_for():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].append(A())
    db["k"].append(B())
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_append_get():
    da = defaultdict(list)
    db = defaultdict(list)
    da.get("k").append(A())
    db.get("k").append(B())
    return da["k"][0].execute() + db["k"][0].run()


def use_collections_ddf():
    da = collections.defaultdict(list)
    db = collections.defaultdict(list)
    da["k"].append(A())
    db["k"].append(B())
    return da["k"][0].execute() + db["k"][0].run()


def use_preserves_b():
    db = defaultdict(list)
    db["k"].append(B())
    return db["k"][0].run()
