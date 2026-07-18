from collections import OrderedDict, ChainMap, deque
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_fs_for():
    da = {"k": frozenset([A()])}
    db = {"k": frozenset([B()])}
    n = 0
    for a in da["k"]:
        n += a.run()
    for b in db["k"]:
        n += b.run()
    return n


def use_fs_next():
    da = {"k": frozenset({A()})}
    db = {"k": frozenset({B()})}
    return next(iter(da["k"])).run() + next(iter(db["k"])).run()


def use_fs_var():
    da = {"k": frozenset([A()])}
    db = {"k": frozenset([B()])}
    ga = da["k"]
    gb = db["k"]
    return next(iter(ga)).run() + next(iter(gb)).run()


def use_set_call_for():
    da = {"k": set([A()])}
    db = {"k": set([B()])}
    n = 0
    for a in da["k"]:
        n += a.run()
    for b in db["k"]:
        n += b.run()
    return n


def use_deque_for():
    da = {"k": deque([A()])}
    db = {"k": deque([B()])}
    n = 0
    for a in da["k"]:
        n += a.run()
    for b in db["k"]:
        n += b.run()
    return n


def use_deque_sub():
    da = {"k": deque([A()])}
    db = {"k": deque([B()])}
    return da["k"][0].run() + db["k"][0].run()


def use_deque_var():
    da = {"k": deque([A()])}
    db = {"k": deque([B()])}
    ga = da["k"]
    gb = db["k"]
    return ga[0].run() + gb[0].run()


def use_coll_deque():
    da = {"k": collections.deque([A()])}
    db = {"k": collections.deque([B()])}
    return da["k"][0].run() + db["k"][0].run()


def use_od_fs_lit():
    da = OrderedDict({"k": frozenset([A()])})
    db = OrderedDict({"k": frozenset([B()])})
    return next(iter(da["k"])).run() + next(iter(db["k"])).run()


def use_cm_deque():
    da = ChainMap({"k": deque([A()])})
    db = ChainMap({"k": deque([B()])})
    return da["k"][0].run() + db["k"][0].run()


def use_dict_fs_kw():
    da = dict(k=frozenset([A()]))
    db = dict(k=frozenset([B()]))
    return next(iter(da["k"])).run() + next(iter(db["k"])).run()


def use_comp_fs():
    da = {k: frozenset([A()]) for k in ("k",)}
    db = {k: frozenset([B()]) for k in ("k",)}
    return next(iter(da["k"])).run() + next(iter(db["k"])).run()


def use_preserves_b():
    db = {"k": deque([B()])}
    return db["k"][0].run()
