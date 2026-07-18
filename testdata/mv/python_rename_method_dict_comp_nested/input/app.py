class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_comp_sub():
    da = {k: [A()] for k in ("k",)}
    db = {k: [B()] for k in ("k",)}
    return da["k"][0].run() + db["k"][0].run()


def use_comp_for():
    da = {k: [A()] for k in ("k",)}
    db = {k: [B()] for k in ("k",)}
    n = 0
    for a in da["k"]:
        n += a.run()
    for b in db["k"]:
        n += b.run()
    return n


def use_comp_var():
    da = {k: [A()] for k in ("k",)}
    db = {k: [B()] for k in ("k",)}
    ga = da["k"]
    gb = db["k"]
    return ga[0].run() + gb[0].run()


def use_comp_values():
    da = {k: [A()] for k in ("k",)}
    db = {k: [B()] for k in ("k",)}
    n = 0
    for ga in da.values():
        n += ga[0].run()
    for gb in db.values():
        n += gb[0].run()
    return n


def use_comp_match():
    da = {k: [A()] for k in ("k",)}
    db = {k: [B()] for k in ("k",)}
    match da:
        case {"k": [xa, *_]}:
            r = xa.run()
    match db:
        case {"k": [xb, *_]}:
            r += xb.run()
    return r


def use_comp_scalar():
    da = {k: A() for k in ("k",)}
    db = {k: B() for k in ("k",)}
    return da["k"].run() + db["k"].run()


def use_comp_scalar_values():
    da = {k: A() for k in ("k",)}
    db = {k: B() for k in ("k",)}
    n = 0
    for a in da.values():
        n += a.run()
    for b in db.values():
        n += b.run()
    return n


def use_preserves_b():
    db = {k: [B()] for k in ("k",)}
    return db["k"][0].run()
