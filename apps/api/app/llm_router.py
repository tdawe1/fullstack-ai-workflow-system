"""
LLM Router: Intelligent routing between LLM providers.

Supports:
- Cost-optimized routing (cheapest provider for task)
- Latency-optimized routing (fastest provider)
- Failover routing (fallback on errors)
- Load balancing across providers
"""

import logging
import random
import time
from typing import Dict, Any, Optional, List
from enum import Enum
from dataclasses import dataclass

logger = logging.getLogger(__name__)


class RoutingStrategy(Enum):
    COST = "cost"           # Cheapest provider
    LATENCY = "latency"     # Fastest provider
    FAILOVER = "failover"   # Try in order, fallback on error
    ROUND_ROBIN = "round_robin"  # Distribute evenly
    RANDOM = "random"       # Random selection


@dataclass
class ProviderHealth:
    """Tracks provider health metrics."""
    name: str
    available: bool = True
    error_count: int = 0
    last_error: Optional[float] = None
    avg_latency_ms: float = 0
    request_count: int = 0
    
    def record_success(self, latency_ms: float):
        self.available = True
        self.request_count += 1
        # Exponential moving average
        alpha = 0.3
        self.avg_latency_ms = alpha * latency_ms + (1 - alpha) * self.avg_latency_ms
    
    def record_error(self):
        self.error_count += 1
        self.last_error = time.time()
        if self.error_count >= 3:
            self.available = False
    
    def maybe_recover(self, recovery_seconds: float = 60):
        """Check if provider should be retried after errors."""
        if not self.available and self.last_error:
            if time.time() - self.last_error > recovery_seconds:
                self.available = True
                self.error_count = 0


# Cost per 1K tokens (approximate, for routing decisions)
PROVIDER_COSTS = {
    "openrouter": {"input": 0.001, "output": 0.002},
    "vertex": {"input": 0.00025, "output": 0.0005},
    "bedrock": {"input": 0.003, "output": 0.015},
    "azure": {"input": 0.001, "output": 0.002},
    "openai": {"input": 0.0015, "output": 0.002},
}


class LLMRouter:
    """Intelligent LLM provider router."""
    
    def __init__(self, providers: List[str], strategy: RoutingStrategy = RoutingStrategy.FAILOVER):
        self.strategy = strategy
        self.providers = providers
        self.health: Dict[str, ProviderHealth] = {
            p: ProviderHealth(name=p) for p in providers
        }
        self._round_robin_index = 0
    
    def select_provider(
        self,
        task_type: str = "general",
        prefer_cost: bool = False,
        prefer_latency: bool = False,
        exclude: Optional[List[str]] = None
    ) -> Optional[str]:
        """
        Select the best provider based on strategy and constraints.
        
        Args:
            task_type: Type of task (affects model choice)
            prefer_cost: Override to prefer cheapest
            prefer_latency: Override to prefer fastest
            exclude: Providers to exclude (e.g., already failed)
            
        Returns:
            Provider name or None if all unavailable
        """
        exclude = exclude or []
        
        # Check for recovery
        for health in self.health.values():
            health.maybe_recover()
        
        # Get available providers
        available = [
            p for p in self.providers 
            if p not in exclude and self.health[p].available
        ]
        
        if not available:
            logger.warning("No LLM providers available")
            return None
        
        # Apply strategy
        if prefer_cost or self.strategy == RoutingStrategy.COST:
            return self._select_cheapest(available)
        elif prefer_latency or self.strategy == RoutingStrategy.LATENCY:
            return self._select_fastest(available)
        elif self.strategy == RoutingStrategy.ROUND_ROBIN:
            return self._select_round_robin(available)
        elif self.strategy == RoutingStrategy.RANDOM:
            return random.choice(available)
        else:  # FAILOVER
            return available[0]
    
    def _select_cheapest(self, available: List[str]) -> str:
        """Select cheapest provider."""
        return min(
            available,
            key=lambda p: PROVIDER_COSTS.get(p, {"input": 999})["input"]
        )
    
    def _select_fastest(self, available: List[str]) -> str:
        """Select fastest provider based on observed latency."""
        return min(
            available,
            key=lambda p: self.health[p].avg_latency_ms or float('inf')
        )
    
    def _select_round_robin(self, available: List[str]) -> str:
        """Round-robin selection."""
        self._round_robin_index = (self._round_robin_index + 1) % len(available)
        return available[self._round_robin_index]
    
    def record_success(self, provider: str, latency_ms: float):
        """Record successful request."""
        if provider in self.health:
            self.health[provider].record_success(latency_ms)
    
    def record_error(self, provider: str):
        """Record failed request."""
        if provider in self.health:
            self.health[provider].record_error()
    
    def get_health_status(self) -> Dict[str, Any]:
        """Get health status of all providers."""
        return {
            p: {
                "available": h.available,
                "avg_latency_ms": round(h.avg_latency_ms, 2),
                "error_count": h.error_count,
                "request_count": h.request_count,
            }
            for p, h in self.health.items()
        }


async def route_llm_request(
    router: LLMRouter,
    prompt: str,
    llm_providers: Dict[str, Any],
    max_retries: int = 3
) -> Dict[str, Any]:
    """
    Route request through providers with failover.
    
    Args:
        router: LLM router instance
        prompt: The prompt to send
        llm_providers: Dict of provider instances
        max_retries: Maximum providers to try
        
    Returns:
        Response from successful provider
    """
    tried = []
    last_error = None
    
    for _ in range(max_retries):
        provider_name = router.select_provider(exclude=tried)
        if not provider_name:
            break
        
        tried.append(provider_name)
        provider = llm_providers.get(provider_name)
        
        if not provider:
            continue
        
        start = time.time()
        try:
            result = await provider.generate(prompt)
            latency = (time.time() - start) * 1000
            router.record_success(provider_name, latency)
            
            return {
                "provider": provider_name,
                "result": result,
                "latency_ms": latency,
            }
            
        except Exception as e:
            logger.warning(f"Provider {provider_name} failed: {e}")
            router.record_error(provider_name)
            last_error = e
    
    raise Exception(f"All providers failed. Last error: {last_error}")
