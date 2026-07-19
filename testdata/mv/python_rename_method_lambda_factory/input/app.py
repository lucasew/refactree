class A:
    def run(self) -> int:
        return 1

class B:
    def run(self) -> int:
        return 2

make_a = lambda: A()
make_b = lambda: B()

def use_direct() -> int:
    return make_a().run() + make_b().run()

def use_assign() -> int:
    a = make_a()
    b = make_b()
    return a.run() + b.run()

def use_preserves_b() -> int:
    return make_b().run()
