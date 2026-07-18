from collections import UserDict
import collections


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_userdict_kw():
    da = UserDict(k=A())
    db = UserDict(k=B())
    return da["k"].run() + db["k"].run()


def use_userdict_var():
    da = UserDict(k=A())
    db = UserDict(k=B())
    xa = da["k"]
    xb = db["k"]
    return xa.run() + xb.run()


def use_userdict_literal():
    da = UserDict({"k": A()})
    db = UserDict({"k": B()})
    return da["k"].run() + db["k"].run()


def use_userdict_values():
    da = UserDict(k=A())
    db = UserDict(k=B())
    n = 0
    for a in da.values():
        n += a.run()
    for b in db.values():
        n += b.run()
    return n


def use_userdict_nested():
    da = UserDict(k=[A()])
    db = UserDict(k=[B()])
    return da["k"][0].run() + db["k"][0].run()


def use_collections_userdict():
    da = collections.UserDict(k=A())
    db = collections.UserDict(k=B())
    return da["k"].run() + db["k"].run()


def use_preserves_b():
    db = UserDict(k=B())
    return db["k"].run()
