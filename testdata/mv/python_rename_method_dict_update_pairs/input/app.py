class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_update_pairs():
    da = {}
    db = {}
    da.update([("k", A())])
    db.update([("k", B())])
    return da["k"].run() + db["k"].run()


def use_update_tuple_pairs():
    da = {}
    db = {}
    da.update((("k", A()),))
    db.update((("k", B()),))
    return da["k"].run() + db["k"].run()


def use_update_pairs_assign():
    da = {}
    db = {}
    da.update([("k", A())])
    db.update([("k", B())])
    xa = da["k"]
    xb = db["k"]
    return xa.run() + xb.run()


def use_update_list_pairs():
    da = {}
    db = {}
    da.update([["k", A()]])
    db.update([["k", B()]])
    return da["k"].run() + db["k"].run()
