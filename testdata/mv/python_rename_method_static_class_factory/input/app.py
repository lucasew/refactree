class A:
    def run(self) -> int:
        return 1

    @staticmethod
    def make():
        return A()

    @classmethod
    def create(cls):
        return cls()

class B:
    def run(self) -> int:
        return 2

    @staticmethod
    def make():
        return B()

    @classmethod
    def create(cls):
        return cls()

def use_static() -> int:
    return A.make().run() + B.make().run()

def use_class() -> int:
    return A.create().run() + B.create().run()

def use_assign() -> int:
    a = A.make()
    b = B.make()
    return a.run() + b.run()

def use_preserves_b() -> int:
    return B.make().run() + B.create().run()
