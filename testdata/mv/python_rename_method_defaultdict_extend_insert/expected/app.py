from collections import defaultdict


class A:
    def execute(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_extend():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].extend([A()])
    db["k"].extend([B()])
    return da["k"][0].execute() + db["k"][0].run()


def use_insert():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].insert(0, A())
    db["k"].insert(0, B())
    return da["k"][0].execute() + db["k"][0].run()


def use_extend_for():
    da = defaultdict(list)
    db = defaultdict(list)
    da["k"].extend([A()])
    db["k"].extend([B()])
    n = 0
    for a in da["k"]:
        n += a.execute()
    for b in db["k"]:
        n += b.run()
    return n


def use_get_insert():
    da = defaultdict(list)
    db = defaultdict(list)
    da.get("k").insert(0, A())
    db.get("k").insert(0, B())
    return da["k"][0].execute() + db["k"][0].run()


def use_preserves_b():
    db = defaultdict(list)
    db["k"].extend([B()])
    db["k"].insert(0, B())
    return db["k"][0].run()
