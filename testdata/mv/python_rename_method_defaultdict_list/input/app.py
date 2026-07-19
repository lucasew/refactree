from collections import defaultdict


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_sub(da: defaultdict[str, list[A]], db: defaultdict[str, list[B]]) -> int:
    return da["k"][0].run() + db["k"][0].run()


def use_get(da: defaultdict[str, list[A]], db: defaultdict[str, list[B]]) -> int:
    return da.get("k")[0].run() + db.get("k")[0].run()


def use_var(da: defaultdict[str, list[A]], db: defaultdict[str, list[B]]) -> int:
    ga = da["k"]
    gb = db["k"]
    return ga[0].run() + gb[0].run()


def use_for(da: defaultdict[str, list[A]], db: defaultdict[str, list[B]]) -> int:
    n = 0
    for a in da["k"]:
        n += a.run()
    for b in db["k"]:
        n += b.run()
    return n


def use_values_for(da: defaultdict[str, list[A]], db: defaultdict[str, list[B]]) -> int:
    n = 0
    for ga in da.values():
        n += ga[0].run()
    for gb in db.values():
        n += gb[0].run()
    return n


def use_preserves_b(db: defaultdict[str, list[B]]) -> int:
    return db["k"][0].run()
