(function() {
    'use strict';

    // --- состояние ---
    const state = {
        currentPage: 1,
        selectedTag: '',
        totalPages: 1,
        totalItems: 0,
        allTags: [],
        articles: [],
        loading: false,
        limit: 10,
    };

    // --- DOM-элементы ---
    const feedContainer = document.getElementById('feedContainer');
    const tagFilterContainer = document.getElementById('tagFilterContainer');
    const clearTagsBtn = document.getElementById('clearTagsBtn');
    const pageNumbersSpan = document.getElementById('pageNumbers');
    const prevPageBtn = document.getElementById('prevPage');
    const nextPageBtn = document.getElementById('nextPage');
    const pageInfoSpan = document.getElementById('pageInfo');
    const itemsInfoSpan = document.getElementById('itemsInfo');
    const totalBadge = document.getElementById('totalBadge');
    const limitSelect = document.getElementById('limitSelect');

    async function fetchFeed(page = 1, limit = 10, tag = '') {
        const params = new URLSearchParams();
        params.append('page', String(page));
        params.append('limit', String(limit));
        if (tag) {
            params.append('tag', tag);
        }

        const url = `/feed?${params.toString()}`;

        const response = await fetch(url);

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        const data = await response.json();

        return {
            articles: data.articles || [],
            totalPages: data.totalPages || 1,
            totalItems: data.totalItems || 0,
            allTags: data.allTags || [],
            currentPage: data.currentPage || page,
            selectedTag: data.selectedTag || tag,
            limit: data.limit || limit,
        };
    }

    // --- рендеринг ---

    function renderFeed(data) {
        const { articles, totalPages, totalItems, allTags, currentPage, selectedTag, limit } = data;

        state.articles = articles;
        state.totalPages = totalPages;
        state.totalItems = totalItems;
        state.allTags = allTags;
        state.currentPage = currentPage;
        state.selectedTag = selectedTag;
        state.limit = limit || state.limit;

        // 1. Рендерим карточки
        if (!articles || articles.length === 0) {
            feedContainer.innerHTML = `<div class="status">📭 Новостей не найдено</div>`;
        } else {
            feedContainer.innerHTML = articles.map(article => renderArticleCard(article)).join('');
        }

        // 2. Рендерим теги-фильтры
        renderTagFilters(allTags, selectedTag);

        // 3. Рендерим пагинацию
        renderPagination(currentPage, totalPages);

        // 4. Обновляем информацию
        totalBadge.textContent = `${totalItems} нов.`;
        pageInfoSpan.textContent = `страница ${currentPage} из ${totalPages}`;
        itemsInfoSpan.textContent = `${articles.length} из ${totalItems} новостей`;
    }

    // --- отрисовка одной статьи ---

    function renderArticleCard(article) {
        // Форматируем дату
        const formattedDate = article.date ? formatDate(article.date) : '';

        // Сокращаем контент до 300 символов
        const shortContent = article.content.length > 300
            ? article.content.substring(0, 300) + '...'
            : article.content;

        return `
            <div class="article-card">
                <div class="article-body">
                    <div class="article-header">
                        <h2 class="article-title">${escapeHtml(article.title)}</h2>
                        ${formattedDate ? `<span class="article-date">${formattedDate}</span>` : ''}
                    </div>
                    
                    <div class="article-content">${escapeHtml(shortContent)}</div>
                    
                    <div class="article-footer">
                        <div class="article-tags">
                            ${(article.tags || []).map(tag =>
            `<span class="article-tag">${escapeHtml(tag)}</span>`
        ).join('')}
                        </div>
                        ${article.link ? `<a href="${escapeHtml(article.link)}" target="_blank" class="read-more">Читать далее →</a>` : ''}
                    </div>
                </div>
            </div>
        `;
    }

    // --- вспомогательные функции для отрисовки ---

    function formatDate(dateString) {
        try {
            const date = new Date(dateString);
            if (isNaN(date.getTime())) return '';

            const now = new Date();
            const diff = Math.floor((now - date) / 1000); // разница в секундах

            if (diff < 60) return 'только что';
            if (diff < 3600) return `${Math.floor(diff / 60)} мин. назад`;
            if (diff < 86400) return `${Math.floor(diff / 3600)} ч. назад`;
            if (diff < 172800) return 'вчера';

            const options = { day: 'numeric', month: 'long', year: 'numeric' };
            return date.toLocaleDateString('ru-RU', options);
        } catch {
            return dateString;
        }
    }

    function renderTagFilters(allTags, selectedTag) {
        if (!allTags || allTags.length === 0) {
            tagFilterContainer.innerHTML = '<span style="color:#8a9aa8; font-size:0.9rem;">нет тегов</span>';
            return;
        }

        tagFilterContainer.innerHTML = allTags.map(tag => `
            <button class="tag-btn ${tag === selectedTag ? 'active' : ''}" data-tag="${escapeHtml(tag)}">${escapeHtml(tag)}</button>
        `).join('');

        document.querySelectorAll('.tag-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const tag = this.dataset.tag;
                if (state.selectedTag === tag) {
                    loadFeed(1, '', state.limit);
                } else {
                    loadFeed(1, tag, state.limit);
                }
            });
        });
    }

    function renderPagination(currentPage, totalPages) {
        prevPageBtn.disabled = currentPage <= 1;
        nextPageBtn.disabled = currentPage >= totalPages;

        let pagesHtml = '';
        const maxVisible = 5;
        let startPage = Math.max(1, currentPage - Math.floor(maxVisible / 2));
        let endPage = Math.min(totalPages, startPage + maxVisible - 1);

        if (endPage - startPage < maxVisible - 1) {
            startPage = Math.max(1, endPage - maxVisible + 1);
        }

        for (let i = startPage; i <= endPage; i++) {
            const active = i === currentPage ? 'active' : '';
            pagesHtml += `<button class="page-btn ${active}" data-page="${i}">${i}</button>`;
        }

        pageNumbersSpan.innerHTML = pagesHtml;

        document.querySelectorAll('#pageNumbers .page-btn').forEach(btn => {
            btn.addEventListener('click', function() {
                const page = parseInt(this.dataset.page);
                if (page !== state.currentPage) {
                    loadFeed(page, state.selectedTag, state.limit);
                }
            });
        });
    }

    // --- загрузка данных ---

    async function loadFeed(page, tag, limit) {
        if (state.loading) return;

        state.loading = true;
        feedContainer.innerHTML = `<div class="loader">⏳ Загрузка...</div>`;

        try {
            const data = await fetchFeed(page, limit, tag);
            renderFeed(data);
        } catch (err) {
            console.error('Ошибка загрузки:', err);
            feedContainer.innerHTML = `
                <div class="status">
                    ⚠️ Ошибка загрузки данных<br>
                    <small style="color:#999;">${escapeHtml(err.message)}</small>
                </div>
            `;
        } finally {
            state.loading = false;
        }
    }

    // --- обработчики ---

    prevPageBtn.addEventListener('click', function() {
        if (state.currentPage > 1) {
            loadFeed(state.currentPage - 1, state.selectedTag, state.limit);
        }
    });

    nextPageBtn.addEventListener('click', function() {
        if (state.currentPage < state.totalPages) {
            loadFeed(state.currentPage + 1, state.selectedTag, state.limit);
        }
    });

    limitSelect.addEventListener('change', function() {
        const newLimit = parseInt(this.value);
        if (newLimit !== state.limit) {
            loadFeed(1, state.selectedTag, newLimit);
        }
    });

    clearTagsBtn.addEventListener('click', function() {
        if (state.selectedTag !== '') {
            loadFeed(1, '', state.limit);
        }
    });

    // --- утилиты ---

    function escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    // --- инициализация ---

    loadFeed(1, '', 10);
})();